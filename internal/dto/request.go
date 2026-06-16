package dto

import "innovation-incubation-platform-backend/internal/model"

type LoginRequest struct {
	Credential string `json:"credential" binding:"required"`
	Password   string `json:"password" binding:"required"`
	Role       string `json:"role" binding:"required"`
}

type RegisterRequest struct {
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required"`
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

type IncubationApplyReq struct {
	CarrierID      uint   `json:"carrier_id"`
	IncubateStart  string `json:"incubate_start"`
	IncubateEnd    string `json:"incubate_end"`
	AgreementFileID *uint `json:"agreement_file_id"`
}

type ChangeApplyReq struct {
	ChangeType    string        `json:"change_type"`
	ChangeContent string        `json:"change_content"`
	NewValue      model.JSONMap `json:"new_value"`
}

type PolicyApplyReq struct {
	FormData model.JSONMap `json:"form_data"`
}

type ReviewReq struct {
	Action  string `json:"action"`
	Comment string `json:"comment"`
}

type CarrierInfoReq struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Address      string `json:"address"`
	Area         string `json:"area"`
	ManagerName  string `json:"manager_name"`
	ContactPhone string `json:"contact_phone"`
	Description  string `json:"description"`
}

type PolicyTemplateReq struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	FormSchema  model.JSONMap `json:"form_schema"`
	TargetRole  string        `json:"target_role"`
}

type PublishPolicyReq struct {
	TemplateID    uint          `json:"template_id"`
	Title         string        `json:"title"`
	Conditions    model.JSONMap `json:"conditions"`
	SubsidyAmount string        `json:"subsidy_amount"`
	StartDate     string        `json:"start_date"`
	EndDate       string        `json:"end_date"`
}

type EnterpriseEditReq struct {
	Name         string `json:"name"`
	Industry     string `json:"industry"`
	Scale        string `json:"scale"`
	Address      string `json:"address"`
	LegalPerson  string `json:"legal_person"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
}

type PerformanceTemplateReq struct {
	Name       string        `json:"name"`
	Year       int           `json:"year"`
	FormSchema model.JSONMap `json:"form_schema"`
}

type PerformanceCampaignReq struct {
	TemplateID uint   `json:"template_id"`
	Name       string `json:"name"`
	Year       int    `json:"year"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
}

type ScoreReq struct {
	Score   float64 `json:"score"`
	Status  string  `json:"status"`
	Comment string  `json:"comment"`
}

type PerformanceSubmitReq struct {
	FormData model.JSONMap `json:"form_data"`
}
