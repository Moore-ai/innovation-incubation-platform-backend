package model

type User struct {
	BaseModel
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	Role         string `gorm:"size:32;not null;index" json:"role"`
	Phone        string `gorm:"size:20;not null" json:"phone"`
	Email        string `gorm:"size:128" json:"email"`
	Status       string `gorm:"size:16;default:active" json:"status"`
}

func (User) TableName() string { return "users" }
