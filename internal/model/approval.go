package model

type Approval struct {
	BaseModel
	TargetType string `gorm:"size:32;not null;index" json:"target_type"` // incubation, major_change, policy, performance
	TargetID   uint   `gorm:"index;not null" json:"target_id"`
	Step       string `gorm:"size:32;default:carrier_review" json:"step"` // carrier_review, gov_review
	Action     string `gorm:"size:16;not null" json:"action"`             // submit, approve, reject, return
	Comment    string `gorm:"type:text" json:"comment"`
	ReviewerID uint   `gorm:"index" json:"reviewer_id"`
}

func (Approval) TableName() string { return "approvals" }
