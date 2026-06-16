package model

type IncubationRecord struct {
	BaseModel
	EnterpriseID    uint   `gorm:"index;not null" json:"enterprise_id"`
	CarrierID       uint   `gorm:"index;not null" json:"carrier_id"`
	IncubateStatus  string `gorm:"size:32;default:in_incubation" json:"incubate_status"`
	IncubateStart   string `gorm:"size:32" json:"incubate_start"`
	IncubateEnd     string `gorm:"size:32" json:"incubate_end"`
	AgreementFileID *uint  `json:"agreement_file_id"`
	AgreementFile   File   `gorm:"foreignKey:AgreementFileID" json:"-"`
	Status          string `gorm:"size:16;default:draft" json:"status"`
	Enterprise      Enterprise `gorm:"foreignKey:EnterpriseID" json:"-"`
	Carrier         Carrier    `gorm:"foreignKey:CarrierID" json:"-"`
}

func (IncubationRecord) TableName() string { return "incubation_records" }
