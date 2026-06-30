package model

import (
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// PGVector 表示 PostgreSQL pgvector 扩展的向量类型
type PGVector []float32

// Scan implements sql.Scanner for pgvector (binary or text format).
// pgvector binary: [2B flags][2B reserved][N x 4B float32 BE]
// pgvector text:   "[0.1,0.2,…]"
func (v *PGVector) Scan(src any) error {
	if src == nil {
		*v = nil
		return nil
	}
	switch s := src.(type) {
	case []byte:
		data := s[4:] // skip flags + reserved
		n := len(data) / 4
		*v = make(PGVector, n)
		for i := range n {
			bits := binary.BigEndian.Uint32(data[i*4 : (i+1)*4])
			(*v)[i] = math.Float32frombits(bits)
		}
		return nil
	case string:
		// format: [0.1,0.2,…]
		trimmed := strings.Trim(s, "[]")
		if trimmed == "" {
			*v = PGVector{}
			return nil
		}
		parts := strings.Split(trimmed, ",")
		vec := make(PGVector, len(parts))
		for i, p := range parts {
			f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
			if err != nil {
				return fmt.Errorf("parse pgvector element: %w", err)
			}
			vec[i] = float32(f)
		}
		*v = vec
		return nil
	default:
		return fmt.Errorf("unsupported pgvector type: %T", src)
	}
}

// Value implements driver.Valuer for pgvector text format like "[0.1,0.2,…]".
func (v PGVector) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	return v.String(), nil
}

// String implements fmt.Stringer, returns "[f1,f2,...]" format.
func (v PGVector) String() string {
	s := make([]string, len(v))
	for i, f := range v {
		s[i] = strconv.FormatFloat(float64(f), 'f', -1, 32)
	}
	return "[" + strings.Join(s, ",") + "]"
}

type PolicyTemplate struct {
	BaseModel
	Name        string  `gorm:"size:255;not null" json:"name"`
	Description string  `gorm:"type:text" json:"description"`
	FormSchema  JSONMap `gorm:"type:jsonb" json:"form_schema"`
}

func (PolicyTemplate) TableName() string { return "policy_templates" }

type Policy struct {
	BaseModel
	TargetRole      TargetRole         `gorm:"size:32;not null" json:"target_role"`
	Title           string             `gorm:"size:255;not null" json:"title"`
	Department      string             `gorm:"size:64" json:"department"`
	Requirements    *PolicyRequirement `gorm:"type:jsonb" json:"requirements"`
	StartDate       string             `gorm:"size:32" json:"start_date"`
	EndDate         string             `gorm:"size:32" json:"end_date"`
	Status          PolicyStatus       `gorm:"size:16;default:draft" json:"status"`
	PublishedAt     *time.Time         `json:"published_at"`
	Embedding       PGVector           `gorm:"type:vector(1024)" json:"-"`
	ExtractedFields *ExtractedPolicy   `gorm:"type:jsonb" json:"extracted_fields"`
	ChangeLog       []string           `gorm:"type:jsonb;default:'[]';serializer:json" json:"change_log"`
}

type SubsidyDetail struct {
	Condition string   `json:"condition"`
	Amount    string   `json:"amount"`
	AmountMin *float64 `json:"amount_min,omitempty"`
	AmountMax *float64 `json:"amount_max,omitempty"`
}

// ExtractedPolicy AI 从政策内容中提取的结构化检索字段
type ExtractedPolicy struct {
	PolicyName           string          `json:"policy_name"`
	PolicySummary        string          `json:"policy_summary"`
	ApplicableIndustries []string        `json:"applicable_industries"`
	ApplicableScales     []string        `json:"applicable_scales"`
	ApplicableStatus     string          `json:"applicable_status"`
	SubsidyType          string          `json:"subsidy_type"`
	Subsidies            []SubsidyDetail `json:"subsidies"`
	ApplicableRegion     string          `json:"applicable_region"`
	RequiredDocuments    []string        `json:"required_documents"`
}

// Scan implements sql.Scanner for JSONB deserialization.
func (e *ExtractedPolicy) Scan(src any) error {
	if src == nil {
		*e = ExtractedPolicy{}
		return nil
	}
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, e)
	case string:
		return json.Unmarshal([]byte(v), e)
	default:
		return fmt.Errorf("unsupported type: %T", src)
	}
}

// Value implements driver.Valuer for JSONB serialization.
func (e *ExtractedPolicy) Value() (driver.Value, error) {
	if e == nil {
		return nil, nil
	}
	return json.Marshal(e)
}

func (Policy) TableName() string { return "policies" }

type MaterialFileItem struct {
	Name    string `json:"name"`     // 材料名称
	FileIDs []uint `json:"file_ids"` // 已选择的文件ID列表
}

type PolicyApplication struct {
	BaseModel
	PolicyID      uint               `gorm:"index;not null" json:"policy_id"`
	ApplicantID   uint               `gorm:"index;not null" json:"applicant_id"`
	ApplicantType ApplicantType      `gorm:"size:16;not null" json:"applicant_type"` // enterprise, carrier
	Materials     []MaterialFileItem `gorm:"type:jsonb;column:form_data" json:"materials"`
	Status        ApprovalStatus     `gorm:"size:32;default:draft" json:"status"` // draft, pending, carrier_review, gov_review, approved, rejected, returned
	Policy        Policy             `gorm:"foreignKey:PolicyID" json:"-"`
}

func (PolicyApplication) TableName() string { return "policy_applications" }

type ContactMethod struct {
	Type  ContactMethodType `json:"type"`  // 联系方式类型：phone, email, address, wechat, website, other
	Value string            `json:"value"` // 联系方式值：电话号码、邮箱地址等
	Note  string            `json:"note"`  // 备注说明，如"数据资源处：0551-62999897"
}

// PolicyRequirement 政策要求 — 对应 policies.requirements jsonb 列
type PolicyRequirement struct {
	FulfillmentCriteria  *string               `json:"fulfillment_criteria,omitempty"`
	ApplicationCondition *string               `json:"application_condition,omitempty"`
	ApplicationMaterials []ApplicationMaterial `json:"application_materials,omitempty"`
	Process              *string               `json:"process,omitempty"`
	LegalBasis           []LegalBasisFile      `json:"legal_basis,omitempty"`
	ContactMethods       []ContactMethod       `json:"contact_methods,omitempty"`
}

func (r *PolicyRequirement) Scan(src any) error {
	if src == nil {
		*r = PolicyRequirement{}
		return nil
	}
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, r)
	case string:
		return json.Unmarshal([]byte(v), r)
	default:
		return fmt.Errorf("unsupported type: %T", src)
	}
}

func (r *PolicyRequirement) Value() (driver.Value, error) {
	if r == nil {
		return nil, nil
	}
	return json.Marshal(r)
}

type ApplicationMaterial struct {
	Name             string          `json:"name"`
	Necessity        NecessityType   `json:"necessity"`                   // "necessary" / "unnecessary"
	Remark           string          `json:"remark,omitempty"`            // 备注
	MaterialFormats  []string        `json:"material_formats,omitempty"`  // 支持的文件拓展名，如 ["PDF","XLSX"]；为空表示无限制
	MaterialTemplate *PolicyTemplate `json:"material_template,omitempty"` // 申材料的填报模板，为空表示不额外需要模板
}

type LegalBasisFile struct {
	Title          string `json:"title"`
	SpecificClause string `json:"specific_clause"`
	FileID         uint   `json:"file_id"`
}
