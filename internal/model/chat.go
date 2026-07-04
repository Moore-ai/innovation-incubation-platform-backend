package model

import "time"

type ChatSession struct {
	BaseModel
	UserID        uint      `gorm:"index" json:"user_id"`
	Title         string    `gorm:"size:128" json:"title"`
	LastMessageAt time.Time `json:"last_message_at"`
	MessageCount  int       `gorm:"default:0" json:"message_count"`
}

type ChatMessage struct {
	BaseModel
	SessionID  uint      `gorm:"index;not null" json:"session_id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Role       string    `gorm:"size:16;not null" json:"role"`
	Content    string    `gorm:"type:text;not null;default:''" json:"content"`
	ToolCallID string    `gorm:"size:64" json:"tool_call_id"`
	ToolCalls  string    `gorm:"type:jsonb" json:"tool_calls"`
	Embedding  []float32 `gorm:"type:vector(1024)" json:"-"`
}
