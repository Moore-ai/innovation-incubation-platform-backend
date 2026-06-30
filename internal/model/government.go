package model

type Government struct {
	BaseModel
	UserID     uint   `gorm:"index;not null" json:"user_id"`
	Name       string `gorm:"size:64" json:"name"`
	Department string `gorm:"size:64" json:"department"`
}

func (Government) TableName() string { return "governments" }
