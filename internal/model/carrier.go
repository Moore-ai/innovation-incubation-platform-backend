package model

type Carrier struct {
	BaseModel
	UserID       uint   `gorm:"uniqueIndex;not null" json:"user_id"`
	Name         string `gorm:"size:255;not null" json:"name"`
	Type         string `gorm:"size:64" json:"type"`
	Address      string `gorm:"size:255" json:"address"`
	Area         string `gorm:"size:128" json:"area"`
	ManagerName  string `gorm:"size:64" json:"manager_name"`
	ContactPhone string `gorm:"size:20" json:"contact_phone"`
	Description  string `gorm:"type:text" json:"description"`
	User         User   `gorm:"foreignKey:UserID" json:"-"`
}

func (Carrier) TableName() string { return "carriers" }
