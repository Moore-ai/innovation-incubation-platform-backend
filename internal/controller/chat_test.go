package controller

import (
	"errors"
	"testing"

	"innovation-incubation-platform-backend/config"
)

func TestChatControllerPublicAIError(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		ctl := NewChatController(nil, &config.Config{})
		got := ctl.publicAIError(errors.New("upstream failed"))
		if got != "AI 服务未配置，请联系管理员检查模型 API Key。" {
			t.Fatalf("unexpected message: %s", got)
		}
	})

	t.Run("upstream auth failure", func(t *testing.T) {
		ctl := NewChatController(nil, &config.Config{})
		ctl.cfg.AI.OpenAI.APIKey = "configured"
		got := ctl.publicAIError(errors.New("error, status code: 401, status: 401 Unauthorized, body: Authentication Fails (governor)"))
		if got != "AI 服务鉴权失败，请联系管理员检查模型 API Key。" {
			t.Fatalf("unexpected message: %s", got)
		}
	})
}
