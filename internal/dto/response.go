package dto

type LoginResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

type UserInfo struct {
	ID    uint   `json:"id"`
	Role  string `json:"role"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}
