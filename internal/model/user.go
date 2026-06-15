package model

type User struct {
	BaseModel
	Username     string `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	Role         string `gorm:"size:32;not null;index" json:"role"` // enterprise, carrier, government
	Phone        string `gorm:"size:20" json:"phone"`
	Email        string `gorm:"size:128" json:"email"`
	Status       string `gorm:"size:16;default:active" json:"status"` // active, disabled
}

func (User) TableName() string { return "users" }
