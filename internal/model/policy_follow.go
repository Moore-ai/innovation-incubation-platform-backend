package model

type PolicyFollow struct {
	BaseModel
	EnterpriseID uint   `gorm:"index;not null;uniqueIndex:idx_ent_policy" json:"enterprise_id"`
	PolicyID     uint   `gorm:"index;not null;uniqueIndex:idx_ent_policy" json:"policy_id"`
	Policy       Policy `gorm:"foreignKey:PolicyID" json:"-"`
}

func (PolicyFollow) TableName() string { return "policy_follows" }
