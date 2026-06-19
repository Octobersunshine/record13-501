package model

import (
	"time"

	"gorm.io/gorm"
)

type ContractStatus string

const (
	StatusDraft       ContractStatus = "draft"
	StatusPendingSign ContractStatus = "pending_sign"
	StatusSigned      ContractStatus = "signed"
	StatusRejected    ContractStatus = "rejected"
	StatusExpired     ContractStatus = "expired"
	StatusCancelled   ContractStatus = "cancelled"
)

func ValidStatuses() []ContractStatus {
	return []ContractStatus{
		StatusDraft, StatusPendingSign, StatusSigned,
		StatusRejected, StatusExpired, StatusCancelled,
	}
}

func IsValidStatus(s ContractStatus) bool {
	for _, v := range ValidStatuses() {
		if v == s {
			return true
		}
	}
	return false
}

type Contract struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Title         string         `gorm:"size:255;not null" json:"title"`
	Content       string         `gorm:"type:text;not null" json:"content"`
	Initiator     string         `gorm:"size:128;not null" json:"initiator"`
	Signer        string         `gorm:"size:128;not null" json:"signer"`
	Status        ContractStatus `gorm:"size:32;not null;default:draft" json:"status"`
	RejectReason  string         `gorm:"type:text" json:"reject_reason,omitempty"`
	SignURL       string         `gorm:"size:512" json:"sign_url,omitempty"`
	SignatureImage string        `gorm:"size:512" json:"signature_image,omitempty"`
	EffectiveDate *time.Time     `json:"effective_date,omitempty"`
	ExpiryDate    *time.Time     `json:"expiry_date,omitempty"`
	SignedAt      *time.Time     `json:"signed_at,omitempty"`
	RejectedAt    *time.Time     `json:"rejected_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Contract) TableName() string {
	return "contracts"
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&Contract{})
}
