package model

type IncubationRecord struct {
	BaseModel
	EnterpriseID   uint       `gorm:"index;not null" json:"enterprise_id"`
	CarrierID      uint       `gorm:"index;not null" json:"carrier_id"`
	IncubateStatus string     `gorm:"size:32;default:in_incubation" json:"incubate_status"` // in_incubation, graduated, moved_out, other
	IncubateStart  string     `gorm:"size:32" json:"incubate_start"`
	IncubateEnd    string     `gorm:"size:32" json:"incubate_end"`
	AgreementFile  string     `gorm:"size:255" json:"agreement_file"`
	Status         string     `gorm:"size:16;default:draft" json:"status"` // draft, pending, approved, rejected, returned
	Enterprise     Enterprise `gorm:"foreignKey:EnterpriseID" json:"-"`
	Carrier        Carrier    `gorm:"foreignKey:CarrierID" json:"-"`
}

func (IncubationRecord) TableName() string { return "incubation_records" }
