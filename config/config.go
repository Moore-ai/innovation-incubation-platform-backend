package config

import (
	"log/slog"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	DB     DBConfig     `mapstructure:"db"`
	JWT    JWTConfig    `mapstructure:"jwt"`
	AI     AIConfig     `mapstructure:"ai"`
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
