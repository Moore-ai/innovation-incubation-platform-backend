package config

import (
	"log/slog"
	"os"
	"slices"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	DB     DBConfig     `mapstructure:"db"`
	JWT      JWTConfig        `mapstructure:"jwt"`
	AI       AIConfig         `mapstructure:"ai"`
	Redis    RedisConfig      `mapstructure:"redis"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
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
	Provider  string          `mapstructure:"provider"`
	Anthropic AnthropicConfig `mapstructure:"anthropic"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
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

type AnthropicConfig struct {
	APIKey         string `mapstructure:"api_key"`
	BaseURL        string `mapstructure:"base_url"`
	Model          string `mapstructure:"model"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}

func MustLoad(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	return cfg
}

func Load(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
