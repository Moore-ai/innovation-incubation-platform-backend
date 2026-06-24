package model

import "time"

type PolicyTemplate struct {
	BaseModel
	Name        string     `gorm:"size:255;not null" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	FormSchema  JSONMap    `gorm:"type:jsonb" json:"form_schema"`
	TargetRole  TargetRole `gorm:"size:32;not null" json:"target_role"` // enterprise, carrier, both
}

func (PolicyTemplate) TableName() string { return "policy_templates" }

type Policy struct {
	BaseModel
	TemplateID      uint               `gorm:"index;not null" json:"template_id"`
	Title           string             `gorm:"size:255;not null" json:"title"`
	Requirements    *PolicyRequirement `gorm:"type:jsonb" json:"requirements"`
	StartDate       string             `gorm:"size:32" json:"start_date"`
	EndDate         string             `gorm:"size:32" json:"end_date"`
	Status          PolicyStatus       `gorm:"size:16;default:draft" json:"status"`
	PublishedAt     *time.Time         `json:"published_at"`
	ExtractedFields JSONMap            `gorm:"type:jsonb" json:"extracted_fields"`
	MatchLevel      string             `gorm:"-" json:"match_level,omitempty"`
}

func (Policy) TableName() string { return "policies" }

type PolicyApplication struct {
	BaseModel
	PolicyID      uint           `gorm:"index;not null" json:"policy_id"`
	ApplicantID   uint           `gorm:"index;not null" json:"applicant_id"`
	ApplicantType ApplicantType  `gorm:"size:16;not null" json:"applicant_type"` // enterprise, carrier
	FormData      JSONMap        `gorm:"type:jsonb" json:"form_data"`
	Status        ApprovalStatus `gorm:"size:32;default:draft" json:"status"` // draft, pending, carrier_review, gov_review, approved, rejected, returned
	Policy        Policy         `gorm:"foreignKey:PolicyID" json:"-"`
}

func (PolicyApplication) TableName() string { return "policy_applications" }

// PolicyRequirement 政策要求 — 对应 policies.requirements jsonb 列
type PolicyRequirement struct {
	FulfillmentCriteria  *string               `json:"fulfillment_criteria,omitempty"`
	ApplicationCondition *string               `json:"application_condition,omitempty"`
	ApplicationMaterials []ApplicationMaterial `json:"application_materials,omitempty"`
	Process              *string               `json:"process,omitempty"`
	LegalBasis           []LegalBasisFile      `json:"legal_basis,omitempty"`
}

type ApplicationMaterial struct {
	Name             string          `json:"name"`
	Necessity        NecessityType   `json:"necessity"`                   // "necessary" / "unnecessary"
	MaterialFormats  []string        `json:"material_formats,omitempty"`  // 支持的文件拓展名，如 ["PDF","XLSX"]；为空表示无限制
	MaterialTemplate *PolicyTemplate `json:"material_template,omitempty"` // 申材料的填报模板，为空表示不额外需要模板
}

type LegalBasisFile struct {
	Title          string `json:"title"`
	SpecificClause string `json:"specific_clause"`
	FileID         uint   `json:"file_id"`
}
