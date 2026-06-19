package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"econtract/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NotificationHandler struct {
	DB *gorm.DB
}

func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{DB: db}
}

type BatchSignReminderReq struct {
	ContractIDs []uint `json:"contract_ids"`
}

type BatchSignReminderResult struct {
	Total      int      `json:"total"`
	Success    int      `json:"success"`
	Failed     int      `json:"failed"`
	Recipients []string `json:"recipients"`
}

func (h *NotificationHandler) BatchSignReminder(c *gin.Context) {
	var req BatchSignReminderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	db := h.DB.Model(&model.Contract{}).Where("status = ?", model.StatusPendingSign)
	if len(req.ContractIDs) > 0 {
		db = db.Where("id IN ?", req.ContractIDs)
	}

	var contracts []model.Contract
	if err := db.Find(&contracts).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询待签署合同失败")
		return
	}

	if len(contracts) == 0 {
		ok(c, BatchSignReminderResult{Total: 0, Success: 0, Failed: 0, Recipients: []string{}})
		return
	}

	success := 0
	failed := 0
	recipients := make(map[string]bool)

	for _, contract := range contracts {
		title := fmt.Sprintf("【签署提醒】%s", contract.Title)
		content := fmt.Sprintf("您有待签署的合同《%s》，请尽快完成签署。合同编号：%d", contract.Title, contract.ID)
		if contract.SignURL != "" {
			content += fmt.Sprintf(" 签署链接：%s", contract.SignURL)
		}

		notif := model.Notification{
			ContractID: contract.ID,
			Recipient:  contract.Signer,
			Type:       model.NotifyTypeSignReminder,
			Title:      title,
			Content:    content,
			IsRead:     false,
		}

		if err := h.DB.Create(&notif).Error; err != nil {
			failed++
			continue
		}
		success++
		recipients[contract.Signer] = true
	}

	recipientList := make([]string, 0, len(recipients))
	for r := range recipients {
		recipientList = append(recipientList, r)
	}

	ok(c, BatchSignReminderResult{
		Total:      len(contracts),
		Success:    success,
		Failed:     failed,
		Recipients: recipientList,
	})
}

type NotificationListQuery struct {
	Recipient string `form:"recipient"`
	IsRead    *bool  `form:"is_read"`
	Page      int    `form:"page,default=1"`
	PageSize  int    `form:"page_size,default=20"`
}

type NotificationListResult struct {
	Total int64                 `json:"total"`
	Items []model.Notification  `json:"items"`
}

func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	var q NotificationListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		fail(c, http.StatusBadRequest, "查询参数错误")
		return
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}

	db := h.DB.Model(&model.Notification{})
	if q.Recipient != "" {
		db = db.Where("recipient = ?", q.Recipient)
	}
	if q.IsRead != nil {
		db = db.Where("is_read = ?", *q.IsRead)
	}

	var total int64
	db.Count(&total)

	var notifications []model.Notification
	offset := (q.Page - 1) * q.PageSize
	if err := db.Order("id DESC").Offset(offset).Limit(q.PageSize).Find(&notifications).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}

	ok(c, NotificationListResult{Total: total, Items: notifications})
}

func (h *NotificationHandler) GetNotification(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的通知ID")
		return
	}

	var notif model.Notification
	if err := h.DB.First(&notif, id).Error; err != nil {
		fail(c, http.StatusNotFound, "通知不存在")
		return
	}

	ok(c, notif)
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的通知ID")
		return
	}

	now := time.Now()
	result := h.DB.Model(&model.Notification{}).Where("id = ?", id).
		Updates(map[string]interface{}{"is_read": true, "read_at": now})
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, "标记已读失败")
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, "通知不存在")
		return
	}

	var notif model.Notification
	h.DB.First(&notif, id)
	ok(c, notif)
}

type MarkAllReadReq struct {
	Recipient string `json:"recipient"`
}

type MarkAllReadResult struct {
	Updated int64 `json:"updated"`
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	var req MarkAllReadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}
	if req.Recipient == "" {
		fail(c, http.StatusBadRequest, "recipient 不能为空")
		return
	}

	now := time.Now()
	result := h.DB.Model(&model.Notification{}).
		Where("recipient = ? AND is_read = ?", req.Recipient, false).
		Updates(map[string]interface{}{"is_read": true, "read_at": now})
	if result.Error != nil {
		fail(c, http.StatusInternalServerError, "批量标记已读失败")
		return
	}

	ok(c, MarkAllReadResult{Updated: result.RowsAffected})
}

type UnreadCountResult struct {
	Count int64 `json:"count"`
}

func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	recipient := c.Query("recipient")
	if recipient == "" {
		fail(c, http.StatusBadRequest, "recipient 参数不能为空")
		return
	}

	var count int64
	h.DB.Model(&model.Notification{}).
		Where("recipient = ? AND is_read = ?", recipient, false).
		Count(&count)

	ok(c, UnreadCountResult{Count: count})
}

func (h *NotificationHandler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/notifications")
	{
		g.POST("/batch-sign-reminder", h.BatchSignReminder)
		g.GET("", h.ListNotifications)
		g.GET("/unread-count", h.UnreadCount)
		g.GET("/:id", h.GetNotification)
		g.PUT("/:id/read", h.MarkRead)
		g.PUT("/read-all", h.MarkAllRead)
	}
}
