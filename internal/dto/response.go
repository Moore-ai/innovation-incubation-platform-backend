package dto

import "time"

type LoginResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

type UserInfo struct {
	ID         uint   `json:"id"`
	Role       string `json:"role"`
	CreditCode string `json:"credit_code,omitempty"`
	Name       string `json:"name,omitempty"`
	Department string `json:"department,omitempty"`
}

type ChatSessionResp struct {
	ID            uint      `json:"id"`
	Title         string    `json:"title"`
	MessageCount  int       `json:"message_count"`
	LastMessageAt time.Time `json:"last_message_at"`
	CreatedAt     time.Time `json:"created_at"`
}

type ChatMessageResp struct {
	ID         uint      `json:"id"`
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	ToolCalls  string    `json:"tool_calls,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
