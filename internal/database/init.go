package database

import (
	"log/slog"
	"os"

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

	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		slog.Error("failed to auto migrate", "error", err)
		os.Exit(1)
	}

	return db
}
