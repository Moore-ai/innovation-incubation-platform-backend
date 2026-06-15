package dto

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required"` // enterprise, carrier
	Phone    string `json:"phone"`
	Email    string `json:"email"`

	EnterpriseName         string `json:"enterprise_name"`
	EnterpriseCreditCode   string `json:"enterprise_credit_code"`
	EnterpriseIndustry     string `json:"enterprise_industry"`
	EnterpriseScale        string `json:"enterprise_scale"`
	EnterpriseAddress      string `json:"enterprise_address"`

	CarrierName string `json:"carrier_name"`
	CarrierType string `json:"carrier_type"`
	CarrierArea string `json:"carrier_area"`
}
