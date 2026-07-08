package model

type Enterprise struct {
	BaseModel
	UserID               uint    `gorm:"uniqueIndex;not null" json:"user_id"`
	Name                 string  `gorm:"size:255;not null;uniqueIndex" json:"name"`
	CreditCode           string  `gorm:"size:64;uniqueIndex" json:"credit_code"`
	Industry             string  `gorm:"size:64" json:"industry"`
	Scale                string  `gorm:"size:32" json:"scale"`
	Address              string  `gorm:"size:255" json:"address"`
	LegalPerson          string  `gorm:"size:64" json:"legal_person"`
	ContactName          string  `gorm:"size:64" json:"contact_name"`
	ContactPhone         string  `gorm:"size:20" json:"contact_phone"`
	OfficePhone          string  `gorm:"size:32" json:"office_phone"`
	MobilePhone          string  `gorm:"size:20" json:"mobile_phone"`
	OperatingUnitName    string  `gorm:"size:255" json:"operating_unit_name"`
	BankName             string  `gorm:"size:255" json:"bank_name"`
	BankAccount          string  `gorm:"size:64" json:"bank_account"`
	FixedAssetInvestment float64 `json:"fixed_asset_investment"`
	Nature               string  `gorm:"size:64" json:"nature"`
	Type                 string  `gorm:"size:64" json:"type"`
	Level                string  `gorm:"size:64" json:"level"`
	CertificationDate    string  `gorm:"size:20" json:"certification_date"`
	EstablishmentDate    string  `gorm:"size:20" json:"establishment_date"`
	TotalArea            float64 `json:"total_area"`
	FunctionalArea       float64 `json:"functional_area"`
	IncubationArea       float64 `json:"incubation_area"`
	RentArea             float64 `json:"rent_area"`
	RentPrice            float64 `json:"rent_price"`
	WorkstationCount     int     `json:"workstation_count"`
	WorkstationStandard  string  `gorm:"size:255" json:"workstation_standard"`
	ManagersCount        int     `json:"managers_count"`
	TechnicalStaffCount  int     `json:"technical_staff_count"`
	BachelorAboveCount   int     `json:"bachelor_above_count"`
	TrainedStaffCount    int     `json:"trained_staff_count"`
	SeedFundAmount       float64 `json:"seed_fund_amount"`
	SiteProofMaterial     string  `gorm:"size:255" json:"site_proof_material"`
	SeedFundMaterial     string  `gorm:"size:255" json:"seed_fund_material"`
}

func (Enterprise) TableName() string { return "enterprises" }
