package model

// ApprovalStatus — IncubationRecord, MajorChange, PolicyApplication, PerformanceSubmission
type ApprovalStatus string

const (
	ApprovalDraft     ApprovalStatus = "draft"
	ApprovalPending   ApprovalStatus = "pending"
	ApprovalApproved  ApprovalStatus = "approved"
	ApprovalRejected  ApprovalStatus = "rejected"
	ApprovalReturned  ApprovalStatus = "returned"
	ApprovalGovReview ApprovalStatus = "gov_review"
)

// PolicyStatus — Policy
type PolicyStatus string

const (
	PolicyDraft     PolicyStatus = "draft"
	PolicyPublished PolicyStatus = "published"
	PolicyClosed    PolicyStatus = "closed"
)

// IncubateStatus — IncubationRecord.IncubateStatus
type IncubateStatus string

const (
	IncubateInIncubation IncubateStatus = "in_incubation"
	IncubateGraduated    IncubateStatus = "graduated"
)

// ApprovalStep — Approval.Step
type ApprovalStep string

const (
	StepCarrierReview ApprovalStep = "carrier_review"
	StepGovReview     ApprovalStep = "gov_review"
)

// ApprovalAction — Approval.Action
type ApprovalAction string

const (
	ActionSubmit  ApprovalAction = "submit"
	ActionApprove ApprovalAction = "approve"
	ActionReject  ApprovalAction = "reject"
	ActionReturn  ApprovalAction = "return"
)

// TargetRole — PolicyTemplate.TargetRole
type TargetRole string

const (
	RoleEnterprise TargetRole = "enterprise"
	RoleCarrier    TargetRole = "carrier"
	RoleBoth       TargetRole = "both"
)

// TargetType — Approval.TargetType
type TargetType string

const (
	TargetIncubation      TargetType = "incubation"
	TargetMajorChange     TargetType = "major_change"
	TargetPolicy          TargetType = "policy"
	TargetPerformance     TargetType = "performance"
	TargetAccountDeletion TargetType = "account_deletion"
)

// ApplicantType — PolicyApplication.ApplicantType
type ApplicantType string

const (
	ApplicantEnterprise ApplicantType = "enterprise"
	ApplicantCarrier    ApplicantType = "carrier"
)

// NecessityType — ApplicationMaterial.Necessity
type NecessityType string

const (
	NecessityRequired    NecessityType = "必要"
	NecessityNotRequired NecessityType = "非必要"
)
