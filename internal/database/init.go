package database

import (
	"log/slog"
	"os"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/database"
	"golang.org/x/crypto/bcrypt"

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
