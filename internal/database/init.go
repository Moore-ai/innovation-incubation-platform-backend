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

	// 迁移：旧版 files.data 列从 NOT NULL 改为允许 NULL
	db.Exec(`DO $$ BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='files' AND column_name='data' AND is_nullable='NO') THEN
			ALTER TABLE files ALTER COLUMN data DROP NOT NULL;
		END IF;
	END $$;`)

	// 迁移：回填旧版 appeals.applicant_type（基于提交者角色，排除 government）
	db.Exec(`UPDATE appeals SET applicant_type = u.role
		FROM users u WHERE appeals.submitted_by = u.id
		AND u.role IN ('enterprise', 'carrier')
		AND (appeals.applicant_type IS NULL OR appeals.applicant_type = '');`)

	// 迁移：policies 表结构调整 — conditions→requirements，删除 subsidy_amount 和 file_id
	db.Exec(`DO $$ BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='policies' AND column_name='conditions')
			AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='policies' AND column_name='requirements') THEN
			ALTER TABLE policies RENAME COLUMN conditions TO requirements;
		END IF;
	END $$;`)
	db.Exec(`ALTER TABLE policies DROP COLUMN IF EXISTS subsidy_amount;`)
	db.Exec(`ALTER TABLE policies DROP COLUMN IF EXISTS file_id;`)
	// 迁移：回填 target_role（仅当 policy_templates 表存在时执行）
	db.Exec(`DO $$ BEGIN
		IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='policy_templates') THEN
			UPDATE policies SET target_role = pt.target_role
			FROM policy_templates pt WHERE policies.template_id = pt.id;
			UPDATE policies SET target_role = 'enterprise' WHERE target_role IS NULL OR target_role = '';
		END IF;
	END $$;`)
	db.Exec(`ALTER TABLE policies DROP COLUMN IF EXISTS template_id;`)
	db.Exec(`DROP TABLE IF EXISTS policy_templates CASCADE;`)

	return db
}
