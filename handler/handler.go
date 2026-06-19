package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"econtract/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	SignatureDir = "./signatures"
	MaxFileSize  = 5 * 1024 * 1024
)

var allowedExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
}

func init() {
	os.MkdirAll(SignatureDir, 0755)
}

type Handler struct {
	DB *gorm.DB
}

func New(db *gorm.DB) *Handler {
	return &Handler{DB: db}
}

type CreateContractReq struct {
	Title         string  `json:"title" binding:"required"`
	Content       string  `json:"content" binding:"required"`
	Initiator     string  `json:"initiator" binding:"required"`
	Signer        string  `json:"signer" binding:"required"`
	SignURL       string  `json:"sign_url"`
	EffectiveDate *string `json:"effective_date"`
	ExpiryDate    *string `json:"expiry_date"`
}

type UpdateStatusReq struct {
	Status       model.ContractStatus `json:"status" binding:"required"`
	RejectReason string               `json:"reject_reason"`
}

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: data})
}

func fail(c *gin.Context, httpCode int, msg string) {
	c.JSON(httpCode, Response{Code: -1, Message: msg})
}

func (h *Handler) CreateContract(c *gin.Context) {
	var req CreateContractReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	contract := model.Contract{
		Title:     req.Title,
		Content:   req.Content,
		Initiator: req.Initiator,
		Signer:    req.Signer,
		Status:    model.StatusDraft,
		SignURL:   req.SignURL,
	}

	if req.EffectiveDate != nil {
		t, err := time.Parse(time.RFC3339, *req.EffectiveDate)
		if err != nil {
			fail(c, http.StatusBadRequest, "effective_date 格式错误，需 RFC3339")
			return
		}
		contract.EffectiveDate = &t
	}
	if req.ExpiryDate != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiryDate)
		if err != nil {
			fail(c, http.StatusBadRequest, "expiry_date 格式错误，需 RFC3339")
			return
		}
		contract.ExpiryDate = &t
	}

	if err := h.DB.Create(&contract).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建合同失败")
		return
	}

	ok(c, contract)
}

func (h *Handler) GetContract(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的合同ID")
		return
	}

	var contract model.Contract
	if err := h.DB.First(&contract, id).Error; err != nil {
		fail(c, http.StatusNotFound, "合同不存在")
		return
	}

	ok(c, contract)
}

type ListQuery struct {
	Status   string `form:"status"`
	Page     int    `form:"page,default=1"`
	PageSize int    `form:"page_size,default=20"`
}

type ListResult struct {
	Total int64             `json:"total"`
	Items []model.Contract  `json:"items"`
}

func (h *Handler) ListContracts(c *gin.Context) {
	var q ListQuery
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

	db := h.DB.Model(&model.Contract{})
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}

	var total int64
	db.Count(&total)

	var contracts []model.Contract
	offset := (q.Page - 1) * q.PageSize
	if err := db.Order("id DESC").Offset(offset).Limit(q.PageSize).Find(&contracts).Error; err != nil {
		fail(c, http.StatusInternalServerError, "查询失败")
		return
	}

	ok(c, ListResult{Total: total, Items: contracts})
}

var allowedTransitions = map[model.ContractStatus][]model.ContractStatus{
	model.StatusDraft:       {model.StatusPendingSign, model.StatusCancelled},
	model.StatusPendingSign: {model.StatusSigned, model.StatusRejected, model.StatusExpired, model.StatusCancelled},
	model.StatusSigned:      {model.StatusExpired},
	model.StatusRejected:    {model.StatusPendingSign, model.StatusCancelled},
}

func canTransition(from, to model.ContractStatus) bool {
	allowed, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

func saveSignatureFile(c *gin.Context, contractID uint) (string, error) {
	file, err := c.FormFile("signature_image")
	if err != nil {
		if err == http.ErrMissingFile {
			return "", nil
		}
		return "", fmt.Errorf("读取签名文件失败: %v", err)
	}

	if file.Size > MaxFileSize {
		return "", fmt.Errorf("签名文件大小超过限制 5MB")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedExts[ext] {
		return "", fmt.Errorf("不支持的文件格式，仅允许 png/jpg/jpeg/gif")
	}

	filename := fmt.Sprintf("contract_%d_%d%s", contractID, time.Now().UnixNano(), ext)
	savePath := filepath.Join(SignatureDir, filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		return "", fmt.Errorf("保存签名文件失败: %v", err)
	}

	return fmt.Sprintf("/signatures/%s", filename), nil
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "无效的合同ID")
		return
	}

	var req UpdateStatusReq
	contentType := c.ContentType()
	if strings.Contains(contentType, "multipart/form-data") {
		req.Status = model.ContractStatus(c.PostForm("status"))
		req.RejectReason = c.PostForm("reject_reason")
	} else if strings.Contains(contentType, "application/json") {
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
			return
		}
	} else {
		fail(c, http.StatusBadRequest, "Content-Type 必须是 application/json 或 multipart/form-data")
		return
	}

	if !model.IsValidStatus(req.Status) {
		fail(c, http.StatusBadRequest, "无效的状态值")
		return
	}

	var contract model.Contract
	if err := h.DB.First(&contract, id).Error; err != nil {
		fail(c, http.StatusNotFound, "合同不存在")
		return
	}

	if !canTransition(contract.Status, req.Status) {
		fail(c, http.StatusConflict, "不允许从 "+string(contract.Status)+" 转换到 "+string(req.Status))
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status": req.Status,
	}

	if req.Status == model.StatusSigned {
		sigPath, err := saveSignatureFile(c, uint(id))
		if err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if sigPath != "" {
			updates["signature_image"] = sigPath
		}
	}

	switch req.Status {
	case model.StatusSigned:
		updates["signed_at"] = now
	case model.StatusRejected:
		updates["rejected_at"] = now
		updates["reject_reason"] = req.RejectReason
	case model.StatusPendingSign:
		updates["rejected_at"] = nil
		updates["reject_reason"] = ""
	}

	if err := h.DB.Model(&contract).Updates(updates).Error; err != nil {
		fail(c, http.StatusInternalServerError, "更新状态失败")
		return
	}

	h.DB.First(&contract, id)
	ok(c, contract)
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/contracts")
	{
		g.POST("", h.CreateContract)
		g.GET("", h.ListContracts)
		g.GET("/:id", h.GetContract)
		g.PUT("/:id/status", h.UpdateStatus)
	}
}
