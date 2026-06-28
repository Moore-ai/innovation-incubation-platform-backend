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

func (t ApprovalStatus) IsValid() bool {
	switch t {
	case ApprovalDraft, ApprovalPending, ApprovalApproved, ApprovalRejected, ApprovalReturned, ApprovalGovReview:
		return true
	default:
		return false
	}
}

// PolicyStatus — Policy
type PolicyStatus string

const (
	PolicyDraft     PolicyStatus = "draft"
	PolicyPublished PolicyStatus = "published"
	PolicyClosed    PolicyStatus = "closed"
)

func (t PolicyStatus) IsValid() bool {
	switch t {
	case PolicyDraft, PolicyPublished, PolicyClosed:
		return true
	default:
		return false
	}
}

// IncubateStatus — IncubationRecord.IncubateStatus
type IncubateStatus string

const (
	IncubateInIncubation IncubateStatus = "in_incubation"
	IncubateGraduated    IncubateStatus = "graduated"
)

func (t IncubateStatus) IsValid() bool {
	switch t {
	case IncubateInIncubation, IncubateGraduated:
		return true
	default:
		return false
	}
}

// ApprovalStep — Approval.Step
type ApprovalStep string

const (
	StepCarrierReview ApprovalStep = "carrier_review"
	StepGovReview     ApprovalStep = "gov_review"
)

func (t ApprovalStep) IsValid() bool {
	switch t {
	case StepCarrierReview, StepGovReview:
		return true
	default:
		return false
	}
}

// ApprovalAction — Approval.Action
type ApprovalAction string

const (
	ActionSubmit  ApprovalAction = "submit"
	ActionApprove ApprovalAction = "approve"
	ActionReject  ApprovalAction = "reject"
	ActionReturn  ApprovalAction = "return"
)

func (t ApprovalAction) IsValid() bool {
	switch t {
	case ActionSubmit, ActionApprove, ActionReject, ActionReturn:
		return true
	default:
		return false
	}
}

// TargetRole — Policy.TargetRole（政策发布时的目标角色：enterprise / carrier / both）
type TargetRole string

const (
	RoleEnterprise TargetRole = "enterprise"
	RoleCarrier    TargetRole = "carrier"
	RoleBoth       TargetRole = "both"
)

func (t TargetRole) IsValid() bool {
	switch t {
	case RoleEnterprise, RoleCarrier, RoleBoth:
		return true
	default:
		return false
	}
}

// TargetType — Approval.TargetType
type TargetType string

const (
	TargetIncubation      TargetType = "incubation"
	TargetMajorChange     TargetType = "major_change"
	TargetPolicy          TargetType = "policy"
	TargetPerformance     TargetType = "performance"
	TargetAccountDeletion TargetType = "account_deletion"
)

func (t TargetType) IsValid() bool {
	switch t {
	case TargetIncubation, TargetMajorChange, TargetPolicy, TargetPerformance, TargetAccountDeletion:
		return true
	default:
		return false
	}
}

// ApplicantType — PolicyApplication.ApplicantType
type ApplicantType string

const (
	ApplicantEnterprise ApplicantType = "enterprise"
	ApplicantCarrier    ApplicantType = "carrier"
)

func (t ApplicantType) IsValid() bool {
	switch t {
	case ApplicantEnterprise, ApplicantCarrier:
		return true
	default:
		return false
	}
}

// CarrierScale — 载体规模
type CarrierScale string

const (
	CarrierScaleSmall  CarrierScale = "small"
	CarrierScaleMedium CarrierScale = "medium"
	CarrierScaleLarge  CarrierScale = "large"
)

func (s CarrierScale) IsValid() bool {
	switch s {
	case CarrierScaleSmall, CarrierScaleMedium, CarrierScaleLarge:
		return true
	default:
		return false
	}
}

// UserRole — 用户角色（企业/载体）
type UserRole string

const (
	UserRoleEnterprise UserRole = "enterprise"
	UserRoleCarrier    UserRole = "carrier"
)

func (r UserRole) IsValid() bool {
	switch r {
	case UserRoleEnterprise, UserRoleCarrier:
		return true
	default:
		return false
	}
}


// NecessityType — ApplicationMaterial.Necessity
type NecessityType string

const (
	NecessityRequired    NecessityType = "necessary"
	NecessityNotRequired NecessityType = "unnecessary"
)

func (t NecessityType) IsValid() bool {
	switch t {
	case NecessityRequired, NecessityNotRequired:
		return true
	default:
		return false
	}
}

// ContactMethodType — ContactMethod.Type
type ContactMethodType string

const (
	ContactPhone   ContactMethodType = "phone"
	ContactEmail   ContactMethodType = "email"
	ContactAddress ContactMethodType = "address"
	ContactWechat  ContactMethodType = "wechat"
	ContactQQ      ContactMethodType = "qq"
	ContactWebsite ContactMethodType = "website"
	ContactOther   ContactMethodType = "other"
)

func (t ContactMethodType) IsValid() bool {
	switch t {
	case ContactPhone, ContactEmail, ContactAddress, ContactWechat, ContactQQ, ContactWebsite, ContactOther:
		return true
	default:
		return false
	}
}
