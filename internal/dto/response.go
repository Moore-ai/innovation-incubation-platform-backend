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

type DictItem struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type DictResponse struct {
	EnterpriseNatures   []DictItem `json:"enterprise_natures"`
	FoundingCategories  []DictItem `json:"founding_categories"`
	IndustryCategories  []DictItem `json:"industry_categories"`
	HighTechFields      []DictItem `json:"high_tech_fields"`
	IncubateStatuses    []DictItem `json:"incubate_statuses"`
}
