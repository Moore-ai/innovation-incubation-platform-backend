package model

type NotificationType string

const (
	NotifIncubationPending          NotificationType = "incubation_pending"
	NotifIncubationReviewed         NotificationType = "incubation_reviewed"
	NotifChangePending              NotificationType = "change_pending"
	NotifChangeReviewed             NotificationType = "change_reviewed"
	NotifApplicationPending         NotificationType = "application_pending"
	NotifApplicationCarrierApproved NotificationType = "application_carrier_approved"
	NotifApplicationReviewed        NotificationType = "application_reviewed"
	NotifPerformanceSubmitted       NotificationType = "performance_submitted"
	NotifPerformanceScored          NotificationType = "performance_scored"
	NotifPolicyPublished            NotificationType = "policy_published"
)

type Notification struct {
	BaseModel
	UserID     uint             `gorm:"index;not null" json:"user_id"`
	Type       NotificationType `gorm:"size:32;not null" json:"type"`
	Title      string           `gorm:"size:255;not null" json:"title"`
	Content    string           `gorm:"type:text" json:"content"`
	TargetType TargetType `gorm:"size:32" json:"target_type"`
	TargetID   uint             `json:"target_id"`
}

func (Notification) TableName() string { return "notifications" }
