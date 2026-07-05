package dto

type LoginResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

type UserInfo struct {
	ID         uint   `json:"id"`
	Role       string `json:"role"`
	CreditCode string `json:"credit_code,omitempty"`
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
