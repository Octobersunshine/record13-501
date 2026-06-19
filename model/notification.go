package model

import (
	"time"

	"gorm.io/gorm"
)

type NotificationType string

const (
	NotifyTypeSignReminder NotificationType = "sign_reminder"
	NotifyTypeSigned       NotificationType = "signed"
	NotifyTypeRejected     NotificationType = "rejected"
)

type Notification struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	ContractID uint           `gorm:"index;not null" json:"contract_id"`
	Recipient  string         `gorm:"size:128;index;not null" json:"recipient"`
	Type       NotificationType `gorm:"size:32;not null" json:"type"`
	Title      string         `gorm:"size:255;not null" json:"title"`
	Content    string         `gorm:"type:text;not null" json:"content"`
	IsRead     bool           `gorm:"default:false;index" json:"is_read"`
	ReadAt     *time.Time     `json:"read_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Notification) TableName() string {
	return "notifications"
}
