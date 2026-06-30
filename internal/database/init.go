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

	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		slog.Error("failed to auto migrate", "error", err)
		os.Exit(1)
	}

	return db
}
