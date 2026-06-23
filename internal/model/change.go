package model

import (
	"database/sql/driver"
	"encoding/json"
)

type JSONMap map[string]any

func (j JSONMap) Value() (driver.Value, error) {
	b, _ := json.Marshal(j)
	return b, nil
}

func (j *JSONMap) Scan(value any) error {
	return json.Unmarshal(value.([]byte), j)
}

type MajorChange struct {
	BaseModel
	EnterpriseID  uint           `gorm:"index;not null" json:"enterprise_id"`
	ChangeType    string         `gorm:"size:64;not null" json:"change_type"`
	ChangeContent string         `gorm:"type:text" json:"change_content"`
	OldValue      JSONMap        `gorm:"type:jsonb" json:"old_value"`
	NewValue      JSONMap        `gorm:"type:jsonb" json:"new_value"`
	Status        ApprovalStatus `gorm:"size:16;default:draft" json:"status"` // draft, pending, approved, rejected, returned
	Enterprise    Enterprise     `gorm:"foreignKey:EnterpriseID" json:"-"`
}

func (MajorChange) TableName() string { return "major_changes" }
