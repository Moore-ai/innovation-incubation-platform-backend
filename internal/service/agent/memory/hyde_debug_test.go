package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/pkg/aiclient"
)

func TestDebugHydeDoc(t *testing.T) {
	cfg := loadConfig(t)
	if cfg.AI.OpenAI.APIKey == "" {
		t.Skip("AI_API_KEY not configured")
	}

	aiClient := aiclient.New(cfg.AI.OpenAI.BaseURL, cfg.AI.OpenAI.APIKey, cfg.AI.OpenAI.Model, 30)
	sm := NewSemanticMemory(nil, aiClient, nil, 3, 256)

	queries := []string{
		"帮我查补贴政策",
		"以后生成报告默认用PDF",
		"你好",
		"查询合肥高新区的企业",
		"回复可以简洁一点吗",
		"你真的好难用呀",
		"生成一份报告，统计合肥地区企业的分布状况，给出发展建议",
	}

	ctx := context.Background()
	for _, q := range queries {
		result := sm.generateHydeDoc(ctx, q)
		fmt.Printf("查询: %s\n", q)
		fmt.Printf("HyDE: %s\n\n", result)
	}
}

func loadConfig(t *testing.T) *config.Config {
	t.Helper()
	dir, _ := os.Getwd()
	for range 10 {
		p := filepath.Join(dir, "config", "config.yaml")
		if _, err := os.Stat(p); err == nil {
			// config.Load 内部使用相对路径加载 prompts，需要切换到项目根目录
			os.Chdir(dir)
			cfg, err := config.Load(p)
			if err != nil {
				t.Fatalf("load config %s: %v", p, err)
			}
			return cfg
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	cwd, _ := os.Getwd()
	t.Fatalf("config.yaml not found (searched upward from %s)", cwd)
	return nil
}
