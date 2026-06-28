package model

type AccountDeletionRequest struct {
	BaseModel
	UserID        uint           `gorm:"index;not null" json:"user_id"`
	Role          string         `gorm:"size:32;not null" json:"role"`
	EnterpriseID  *uint          `json:"enterprise_id,omitempty"`
	CarrierID     *uint          `json:"carrier_id,omitempty"`
	Reason        string         `gorm:"type:text" json:"reason"`
	Status        ApprovalStatus `gorm:"size:16;default:pending" json:"status"`
	ReviewerID    *uint          `json:"reviewer_id,omitempty"`
	ReviewComment string         `gorm:"type:text" json:"review_comment"`
}

func (AccountDeletionRequest) TableName() string { return "account_deletion_requests" }
