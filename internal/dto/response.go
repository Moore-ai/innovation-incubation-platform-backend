package dto

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
