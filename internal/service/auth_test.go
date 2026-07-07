package service

import (
	"testing"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAuthServiceRegisterRejectsGovernmentSelfRegistration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Government{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	svc := NewAuthService(repository.NewAuthRepo(db), config.JWTConfig{Secret: "test-secret"})
	resp, err := svc.Register(&dto.RegisterRequest{
		Role:     string(model.UserRoleGovernment),
		Phone:    "13800138000",
		Password: "password123",
	})

	if err == nil {
		t.Fatalf("expected government registration to be rejected, got response: %#v", resp)
	}

	var users int64
	if countErr := db.Model(&model.User{}).Count(&users).Error; countErr != nil {
		t.Fatalf("count users: %v", countErr)
	}
	if users != 0 {
		t.Fatalf("expected no user to be created, got %d", users)
	}
}
