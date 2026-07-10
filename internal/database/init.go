package database

import (
	"log/slog"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/database"

	"gorm.io/gorm"
)

func MustInit(cfg *config.Config) *gorm.DB {
	db, err := database.NewDB(cfg.DB)
	if err != nil {
		slog.Error("failed to connect database", "error", err)
		os.Exit(1)
	}

	db.Exec("CREATE EXTENSION IF NOT EXISTS vector")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_messages_fts ON chat_messages USING GIN (to_tsvector('simple', content))")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_messages_vector ON chat_messages USING ivfflat (embedding vector_cosine_ops) WITH (lists = 50)")

	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		slog.Error("failed to auto migrate", "error", err)
		os.Exit(1)
	}

	initDemoUsers(db)
	initDemoCarriers(db)
	initDemoPolicyMaterials(db)

	return db
}

// initDemoUsers 创建演示账号
func initDemoUsers(db *gorm.DB) {
	var count int64
	db.Model(&model.User{}).Where("role = ? AND phone = ?", "government", "13800000001").Count(&count)
	if count > 0 {
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	govUser := &model.User{
		Role:         "government",
		Phone:        "13800000001",
		PasswordHash: string(hash),
		Email:        "gov@test.com",
	}
	if err := db.Create(govUser).Error; err != nil {
		slog.Warn("failed to create government demo user", "error", err)
		return
	}

	gov := &model.Government{
		UserID:     govUser.ID,
		Name:       "政务管理部",
		Department: "科技局",
	}
	if err := db.Create(gov).Error; err != nil {
		slog.Warn("failed to create government org", "error", err)
	} else {
		slog.Info("government demo user created", "phone", "13800000001", "password", "123456")
	}
}

type demoCarrier struct {
	Phone        string
	Email        string
	Name         string
	Type         string
	Address      string
	Area         string
	ManagerName  string
	ContactPhone string
	Description  string
	Scale        model.CarrierScale
}

// initDemoCarriers 创建并修复演示载体数据。
func initDemoCarriers(db *gorm.DB) {
	demos := []demoCarrier{
		{
			Phone:        "13800000003",
			Email:        "carrier1@test.com",
			Name:         "合肥高新创业服务中心",
			Type:         "孵化器",
			Address:      "合肥市高新区创新大道2800号",
			Area:         "高新区",
			ManagerName:  "张敏",
			ContactPhone: "13800000003",
			Description:  "面向科技型中小企业提供办公空间、创业辅导、投融资对接和政策申报服务。",
			Scale:        "medium",
		},
		{
			Phone:        "13800000004",
			Email:        "carrier2@test.com",
			Name:         "蜀山数字经济孵化器",
			Type:         "产业园",
			Address:      "合肥市蜀山区望江西路与创新大道交口",
			Area:         "蜀山区",
			ManagerName:  "李伟",
			ContactPhone: "13800000004",
			Description:  "聚焦软件、人工智能和数字服务企业，提供场地、技术资源和产业合作机会。",
			Scale:        "large",
		},
		{
			Phone:        "13800000005",
			Email:        "carrier3@test.com",
			Name:         "包河科创加速器",
			Type:         "加速器",
			Address:      "合肥市包河区徽州大道与锦绣大道交口",
			Area:         "包河区",
			ManagerName:  "王芳",
			ContactPhone: "13800000005",
			Description:  "服务成长型科创企业，支持成果转化、市场拓展、人才引进和项目申报。",
			Scale:        "medium",
		},
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	for _, demo := range demos {
		var user model.User
		err := db.Where("role = ? AND phone = ?", string(model.UserRoleCarrier), demo.Phone).First(&user).Error
		if err != nil {
			user = model.User{
				Role:         string(model.UserRoleCarrier),
				Phone:        demo.Phone,
				PasswordHash: string(hash),
				Email:        demo.Email,
			}
			if err := db.Create(&user).Error; err != nil {
				slog.Warn("failed to create carrier demo user", "phone", demo.Phone, "error", err)
				continue
			}
		}

		var carrier model.Carrier
		err = db.Where("user_id = ?", user.ID).First(&carrier).Error
		if err != nil {
			carrier = model.Carrier{UserID: user.ID}
			applyDemoCarrier(&carrier, demo)
			if err := db.Create(&carrier).Error; err != nil {
				slog.Warn("failed to create carrier demo", "phone", demo.Phone, "error", err)
			}
			continue
		}

		if carrierLooksBroken(carrier) || carrier.ContactPhone == demo.Phone {
			applyDemoCarrier(&carrier, demo)
			if err := db.Save(&carrier).Error; err != nil {
				slog.Warn("failed to repair carrier demo", "phone", demo.Phone, "error", err)
			}
		}
	}
}

func applyDemoCarrier(carrier *model.Carrier, demo demoCarrier) {
	carrier.Name = demo.Name
	carrier.Type = demo.Type
	carrier.Address = demo.Address
	carrier.Area = demo.Area
	carrier.ManagerName = demo.ManagerName
	carrier.ContactPhone = demo.ContactPhone
	carrier.Description = demo.Description
	carrier.Scale = demo.Scale
}

func carrierLooksBroken(carrier model.Carrier) bool {
	values := []string{carrier.Name, carrier.Type, carrier.Address, carrier.Area, carrier.ManagerName, carrier.Description}
	for _, value := range values {
		if strings.Contains(value, "?") {
			return true
		}
	}
	return strings.TrimSpace(carrier.Name) == ""
}

func initDemoPolicyMaterials(db *gorm.DB) {
	const title = "培育省级重点工业互联网平台"

	var policy model.Policy
	if err := db.Where("title = ?", title).First(&policy).Error; err != nil {
		slog.Warn("demo prefill policy not found", "title", title, "error", err)
		return
	}
	if policy.Requirements == nil {
		policy.Requirements = &model.PolicyRequirement{}
	}
	if len(policy.Requirements.ApplicationMaterials) > 0 {
		return
	}

	policy.Requirements.ApplicationMaterials = []model.ApplicationMaterial{
		{
			Name:            "营业执照",
			Necessity:       model.NecessityRequired,
			Remark:          "用于核验企业主体资格。",
			MaterialFormats: []string{"PDF", "DOC", "DOCX", "TXT", "JPG", "PNG"},
		},
		{
			Name:            "项目申报书",
			Necessity:       model.NecessityRequired,
			Remark:          "说明工业互联网平台建设内容、投入和预期成效。",
			MaterialFormats: []string{"PDF", "DOC", "DOCX"},
		},
		{
			Name:            "财务报表",
			Necessity:       model.NecessityNotRequired,
			Remark:          "可作为企业经营和投入能力证明。",
			MaterialFormats: []string{"PDF", "XLS", "XLSX", "TXT"},
		},
	}
	if err := db.Model(&model.Policy{}).Where("id = ?", policy.ID).Update("requirements", policy.Requirements).Error; err != nil {
		slog.Warn("failed to init demo policy materials", "policy_id", policy.ID, "error", err)
	}
}
