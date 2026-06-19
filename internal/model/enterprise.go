package model

type Enterprise struct {
	BaseModel
	UserID       uint   `gorm:"uniqueIndex;not null" json:"user_id"`
	Name         string `gorm:"size:255;not null" json:"name"`
	CreditCode   string `gorm:"size:64;uniqueIndex" json:"credit_code"`
	Industry     string `gorm:"size:64" json:"industry"`
	Scale        string `gorm:"size:32" json:"scale"`
	Address      string `gorm:"size:255" json:"address"`
	LegalPerson  string `gorm:"size:64" json:"legal_person"`
	ContactName  string `gorm:"size:64" json:"contact_name"`
	ContactPhone string `gorm:"size:20" json:"contact_phone"`
}

func (Enterprise) TableName() string { return "enterprises" }
