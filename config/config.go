package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	DB        DBConfig        `mapstructure:"db"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	AI        AIConfig        `mapstructure:"ai"`
	Redis     RedisConfig     `mapstructure:"redis"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Upload    UploadConfig    `mapstructure:"upload"`
	Log       LogConfig       `mapstructure:"log"`
	Notification NotificationConfig `mapstructure:"notification"`
}

type NotificationConfig struct {
	HeartbeatSeconds int  `mapstructure:"heartbeat_seconds"`
	RecentCount      int  `mapstructure:"recent_count"`
	MaxConnsPerUser  int  `mapstructure:"max_conns_per_user"`
}

type LogConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type AIConfig struct {
	Provider     string                 `mapstructure:"provider"`
	OpenAI       OpenAICompatibleConfig `mapstructure:"openai"`
	Prompts      PromptsConfig          `mapstructure:"prompts"`
	MaxFileChars int                    `mapstructure:"max_file_chars"`
}

type PromptsConfig struct {
	Extract   string `mapstructure:"extract"`
	Match     string `mapstructure:"match"`
	Prefill   string `mapstructure:"prefill"`
	Summarize string `mapstructure:"summarize"`
}

type OpenAICompatibleConfig struct {
	APIKey         string `mapstructure:"api_key"`
	BaseURL        string `mapstructure:"base_url"`
	Model          string `mapstructure:"model"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type UploadConfig struct {
	MaxSizeMB         int64    `mapstructure:"max_size_mb"`
	Dir               string   `mapstructure:"dir"`
	AllowedExtensions []string `mapstructure:"allowed_extensions"`
}

type RateLimitConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Algorithm  string `mapstructure:"algorithm"`
	DefaultRPM int    `mapstructure:"default_rpm"`
	Whitelist  []uint `mapstructure:"whitelist"`
}

func (c *RateLimitConfig) IsWhitelisted(userID uint) bool {
	return slices.Contains(c.Whitelist, userID)
}

func MustLoad(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	return cfg
}

func loadDotEnv(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func MustLoadPromptFile(path string, target *string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read prompt", "error", err)
		os.Exit(1)
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed != "" {
		*target = trimmed
	}
}

func Load(path string) (*Config, error) {
	loadDotEnv(".env")

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	expanded := os.ExpandEnv(string(raw))

	v := viper.New()
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	if err := v.ReadConfig(bytes.NewReader([]byte(expanded))); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	MustLoadPromptFile("config/prompts/extract.txt", &cfg.AI.Prompts.Extract)
	MustLoadPromptFile("config/prompts/match.txt", &cfg.AI.Prompts.Match)
	MustLoadPromptFile("config/prompts/prefill.txt", &cfg.AI.Prompts.Prefill)
	MustLoadPromptFile("config/prompts/summarize.txt", &cfg.AI.Prompts.Summarize)
	return &cfg, nil
}
