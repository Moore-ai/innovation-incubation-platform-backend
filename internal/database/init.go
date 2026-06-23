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

	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		slog.Error("failed to auto migrate", "error", err)
		os.Exit(1)
	}

	// 迁移：旧版 files.data 列从 NOT NULL 改为允许 NULL
	db.Exec(`DO $$ BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='files' AND column_name='data' AND is_nullable='NO') THEN
			ALTER TABLE files ALTER COLUMN data DROP NOT NULL;
		END IF;
	END $$;`)

	// 迁移：policies 表结构调整 — conditions→requirements，删除 subsidy_amount 和 file_id
	db.Exec(`DO $$ BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='policies' AND column_name='conditions')
			AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='policies' AND column_name='requirements') THEN
			ALTER TABLE policies RENAME COLUMN conditions TO requirements;
		END IF;
	END $$;`)
	db.Exec(`ALTER TABLE policies DROP COLUMN IF EXISTS subsidy_amount;`)
	db.Exec(`ALTER TABLE policies DROP COLUMN IF EXISTS file_id;`)

	return db
}
