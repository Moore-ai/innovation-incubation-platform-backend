package model

type PerformanceTemplate struct {
	BaseModel
	Name       string  `gorm:"size:255;not null" json:"name"`
	Year       int     `gorm:"not null" json:"year"`
	FormSchema JSONMap `gorm:"type:jsonb" json:"form_schema"`
}

func (PerformanceTemplate) TableName() string { return "performance_templates" }

type PerformanceCampaign struct {
	BaseModel
	TemplateID uint                `gorm:"index;not null" json:"template_id"`
	Name       string              `gorm:"size:255;not null" json:"name"`
	Year       int                 `gorm:"not null" json:"year"`
	StartDate  string              `gorm:"size:32" json:"start_date"`
	EndDate    string              `gorm:"size:32" json:"end_date"`
	IsActive   bool                `gorm:"default:false" json:"is_active"`
	Template   PerformanceTemplate `gorm:"foreignKey:TemplateID" json:"-"`
}

func (PerformanceCampaign) TableName() string { return "performance_campaigns" }

type PerformanceSubmission struct {
	BaseModel
	CampaignID uint                `gorm:"index;not null" json:"campaign_id"`
	CarrierID  uint                `gorm:"index;not null" json:"carrier_id"`
	FormData   JSONMap             `gorm:"type:jsonb" json:"form_data"`
	Status     string              `gorm:"size:16;default:draft" json:"status"` // draft, pending, approved, rejected, returned
	Score      *float64            `json:"score"`
	Campaign   PerformanceCampaign `gorm:"foreignKey:CampaignID" json:"-"`
	Carrier    Carrier             `gorm:"foreignKey:CarrierID" json:"-"`
}

func (PerformanceSubmission) TableName() string { return "performance_submissions" }
