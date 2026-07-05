package model

import (
	"time"

	"gorm.io/gorm"
)

type SemanticMemory struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	UserID     *uint          `gorm:"index" json:"user_id"`
	Content    string         `gorm:"type:text;not null" json:"content"`
	Embedding  []float32      `gorm:"type:vector(1024)" json:"-"`
	Importance float64        `gorm:"default:0.5" json:"importance"`
	Category   string         `gorm:"size:64" json:"category"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
