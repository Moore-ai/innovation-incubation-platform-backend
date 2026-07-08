package dto

import "innovation-incubation-platform-backend/internal/model"

type LoginRequest struct {
	Password   string `json:"password" binding:"required"`
	Role       string `json:"role" binding:"required"`
	Phone      string `json:"phone"`
	CreditCode string `json:"credit_code"`
}

type RegisterRequest struct {
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`

	EnterpriseName        string `json:"enterprise_name"`
	EnterpriseCreditCode  string `json:"enterprise_credit_code"`
	EnterpriseIndustry    string `json:"enterprise_industry"`
	EnterpriseScale       string `json:"enterprise_scale"`
	EnterpriseAddress     string `json:"enterprise_address"`
	EnterpriseLegalPerson string `json:"enterprise_legal_person"`
	EnterpriseContactName string `json:"enterprise_contact_name"`

	CarrierName string `json:"carrier_name"`
	CarrierType string `json:"carrier_type"`
	CarrierArea string `json:"carrier_area"`
}

type IncubationApplyReq struct {
	CarrierID       uint   `json:"carrier_id"`
	IncubateStart   string `json:"incubate_start"`
	IncubateEnd     string `json:"incubate_end"`
	AgreementFileID *uint  `json:"agreement_file_id"`
	CreditCode      string `json:"credit_code"`
	EnterpriseName  string `json:"enterprise_name"`
}

type ChangeApplyReq struct {
	ChangeType    string        `json:"change_type"`
	ChangeContent string        `json:"change_content"`
	NewValue      model.JSONMap `json:"new_value"`
}

type PolicyApplyReq struct {
	Materials []model.MaterialFileItem `json:"materials"`
}

type ReviewReq struct {
	Action  string `json:"action"`
	Comment string `json:"comment"`
}

type MarkReadReq struct {
	IDs []uint `json:"ids" binding:"required,min=1"`
}

type CarrierInfoReq struct {
	Name            string             `json:"name"`
	Type            string             `json:"type"`
	Address         string             `json:"address"`
	Area            string             `json:"area"`
	ManagerName     string             `json:"manager_name"`
	ContactPhone    string             `json:"contact_phone"`
	Description     string             `json:"description"`
	Scale           model.CarrierScale `json:"scale"`
	SpecialtyFields []string           `json:"specialty_fields"`
}

type PublishPolicyReq struct {
	TargetRole   string                   `json:"target_role" binding:"required,oneof=enterprise carrier both"`
	Title        string                   `json:"title"`
	Department   string                   `json:"department"`
	Requirements *model.PolicyRequirement `json:"requirements" binding:"required"`
	StartDate    string                   `json:"start_date"`
	EndDate      string                   `json:"end_date"`
}

type EnterpriseEditReq struct {
	Name                 string  `json:"name"`
	CreditCode           string  `json:"credit_code"`
	Industry             string  `json:"industry"`
	Scale                string  `json:"scale"`
	Address              string  `json:"address"`
	LegalPerson          string  `json:"legal_person"`
	ContactName          string  `json:"contact_name"`
	ContactPhone         string  `json:"contact_phone"`
	OfficePhone          string  `json:"office_phone"`
	MobilePhone          string  `json:"mobile_phone"`
	OperatingUnitName    string  `json:"operating_unit_name"`
	BankName             string  `json:"bank_name"`
	BankAccount          string  `json:"bank_account"`
	FixedAssetInvestment float64 `json:"fixed_asset_investment"`
	Nature               string  `json:"nature"`
	Type                 string  `json:"type"`
	Level                string  `json:"level"`
	CertificationDate    string  `json:"certification_date"`
	EstablishmentDate    string  `json:"establishment_date"`
	TotalArea            float64 `json:"total_area"`
	FunctionalArea       float64 `json:"functional_area"`
	IncubationArea       float64 `json:"incubation_area"`
	RentArea             float64 `json:"rent_area"`
	RentPrice            float64 `json:"rent_price"`
	WorkstationCount     int     `json:"workstation_count"`
	WorkstationStandard  string  `json:"workstation_standard"`
	ManagersCount        int     `json:"managers_count"`
	TechnicalStaffCount  int     `json:"technical_staff_count"`
	BachelorAboveCount   int     `json:"bachelor_above_count"`
	TrainedStaffCount    int     `json:"trained_staff_count"`
	SeedFundAmount       float64 `json:"seed_fund_amount"`
	SiteProofMaterial     string  `json:"site_proof_material"`
	SeedFundMaterial     string  `json:"seed_fund_material"`
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

type SubmitAppealReq struct {
	Identifier  string `json:"identifier" binding:"required"`
	ProblemType string `json:"problem_type" binding:"required,oneof=tax financing property utility registration labor construction supervision reward other"`
	Department  string `json:"department"`
	Content     string `json:"content" binding:"required"`
}

type UpdateAppealStatusReq struct {
	Status string `json:"status" binding:"required,oneof=pending processed"`
}

type CreateChatSessionReq struct {
	Title string `json:"title"`
}

type SendChatMessageReq struct {
	Content string         `json:"content" binding:"required"`
	State   map[string]any `json:"state"`
}
