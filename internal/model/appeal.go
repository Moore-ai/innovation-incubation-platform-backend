package model

type ProblemType string

const (
	ProblemTax          ProblemType = "tax"
	ProblemFinancing    ProblemType = "financing"
	ProblemProperty     ProblemType = "property"
	ProblemUtility      ProblemType = "utility"
	ProblemRegistration ProblemType = "registration"
	ProblemLabor        ProblemType = "labor"
	ProblemConstruction ProblemType = "construction"
	ProblemSupervision  ProblemType = "supervision"
	ProblemReward       ProblemType = "reward"
	ProblemOther        ProblemType = "other"
)

var validProblemTypes = map[ProblemType]bool{
	ProblemTax: true, ProblemFinancing: true, ProblemProperty: true,
	ProblemUtility: true, ProblemRegistration: true, ProblemLabor: true,
	ProblemConstruction: true, ProblemSupervision: true, ProblemReward: true,
	ProblemOther: true,
}

func (p ProblemType) IsValid() bool {
	return validProblemTypes[p]
}

type AppealStatus string

const (
	AppealPending   AppealStatus = "pending"
	AppealProcessed AppealStatus = "processed"
)

func (s AppealStatus) IsValid() bool {
	return s == AppealPending || s == AppealProcessed
}

type Appeal struct {
	BaseModel
	Identifier    string        `gorm:"size:64;not null" json:"identifier"`
	ProblemType   ProblemType   `gorm:"size:32;not null" json:"problem_type"`
	Department    string        `gorm:"size:64" json:"department"`
	Content       string        `gorm:"type:text;not null" json:"content"`
	Status        AppealStatus  `gorm:"size:16;default:pending" json:"status"`
	ApplicantType ApplicantType `gorm:"size:16;not null" json:"applicant_type"`
	SubmittedBy   uint          `gorm:"index;not null" json:"submitted_by"`
}
