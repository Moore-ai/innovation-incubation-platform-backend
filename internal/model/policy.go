package model

import "time"

type PolicyTemplate struct {
	BaseModel
	Name        string  `gorm:"size:255;not null" json:"name"`
	Description string  `gorm:"type:text" json:"description"`
	FormSchema  JSONMap `gorm:"type:jsonb" json:"form_schema"`
	TargetRole  string  `gorm:"size:32;not null" json:"target_role"` // enterprise, carrier, both
}

func (PolicyTemplate) TableName() string { return "policy_templates" }

type Policy struct {
	BaseModel
	TemplateID    uint           `gorm:"index;not null" json:"template_id"`
	Title         string         `gorm:"size:255;not null" json:"title"`
	Conditions    JSONMap        `gorm:"type:jsonb" json:"conditions"`
	SubsidyAmount string         `gorm:"size:128" json:"subsidy_amount"`
	StartDate     string         `gorm:"size:32" json:"start_date"`
	EndDate       string         `gorm:"size:32" json:"end_date"`
	Status        string         `gorm:"size:16;default:draft" json:"status"` // draft, published, closed
	PublishedAt   *time.Time     `json:"published_at"`
	Template      PolicyTemplate `gorm:"foreignKey:TemplateID" json:"-"`
}

func (Policy) TableName() string { return "policies" }

type PolicyApplication struct {
	BaseModel
	PolicyID      uint   `gorm:"index;not null" json:"policy_id"`
	ApplicantID   uint   `gorm:"index;not null" json:"applicant_id"`
	ApplicantType string `gorm:"size:16;not null" json:"applicant_type"` // enterprise, carrier
	FormData      JSONMap `gorm:"type:jsonb" json:"form_data"`
	Status        string `gorm:"size:32;default:draft" json:"status"` // draft, pending, carrier_review, gov_review, approved, rejected, returned
	Policy        Policy `gorm:"foreignKey:PolicyID" json:"-"`
}

func (PolicyApplication) TableName() string { return "policy_applications" }
