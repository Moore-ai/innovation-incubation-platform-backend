package middleware

import (
	"log/slog"
	"os"

	"github.com/casbin/casbin/v2"
	"gorm.io/gorm"
)

func MustInitEnforcer(db *gorm.DB) *casbin.Enforcer {
	enforcer, err := NewEnforcer(db)
	if err != nil {
		slog.Error("failed to init casbin enforcer", "error", err)
		os.Exit(1)
	}
	SeedPolicies(enforcer)
	return enforcer
}
