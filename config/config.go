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
	Server       ServerConfig       `mapstructure:"server"`
	DB           DBConfig           `mapstructure:"db"`
	JWT          JWTConfig          `mapstructure:"jwt"`
	AI           AIConfig           `mapstructure:"ai"`
	Redis        RedisConfig        `mapstructure:"redis"`
	RateLimit    RateLimitConfig    `mapstructure:"rate_limit"`
	Upload       UploadConfig       `mapstructure:"upload"`
	Log          LogConfig          `mapstructure:"log"`
	Notification NotificationConfig `mapstructure:"notification"`
	FileMatch    FileMatchConfig    `mapstructure:"filematch"`
	Search       SearchConfig       `mapstructure:"search"`
	FileParser   FileParserConfig   `mapstructure:"file_parser"`
}

type FileParserConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	VenvPath   string `mapstructure:"venv_path"`
	ScriptPath string `mapstructure:"script_path"`
	TimeoutSec int    `mapstructure:"timeout_sec"`
}

type FileMatchConfig struct {
	WeightJaro    float64  `mapstructure:"weight_jaro"`
	WeightKeyword float64  `mapstructure:"weight_keyword"`
	WeightPrefix  float64  `mapstructure:"weight_prefix"`
	Threshold     float64  `mapstructure:"threshold"`
	StopWords     []string `mapstructure:"stop_words"`
}

type NotificationConfig struct {
	HeartbeatSeconds int `mapstructure:"heartbeat_seconds"`
	RecentCount      int `mapstructure:"recent_count"`
	MaxConnsPerUser  int `mapstructure:"max_conns_per_user"`
}

type SearchConfig struct {
	Method     string             `mapstructure:"method"`
	MaxResults int                `mapstructure:"max_results"`
	Vector     VectorSearchConfig `mapstructure:"vector"`
}

type VectorSearchConfig struct {
	TopK        int        `mapstructure:"top_k"`
	MinScore    float64    `mapstructure:"min_score"`
	MaxAnalysis int        `mapstructure:"max_analysis"`
	Rerank      bool       `mapstructure:"rerank"`
	MQE         MQEConfig  `mapstructure:"mqe"`
	HyDE        HyDEConfig `mapstructure:"hyde"`
}

type MQEConfig struct {
	Enabled  bool    `mapstructure:"enabled"`
	NQueries int     `mapstructure:"n_queries"`
	RRFK     float64 `mapstructure:"rrf_k"`
}

type HyDEConfig struct {
	Enabled   bool `mapstructure:"enabled"`
	MaxTokens int  `mapstructure:"max_tokens"`
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
	OpenAI       OpenAICompatibleConfig `mapstructure:"openai"`
	Prompts      PromptsConfig          `mapstructure:"prompts"`
	MaxFileChars int                    `mapstructure:"max_file_chars"`
	Embedding    EmbeddingConfig        `mapstructure:"embedding"`
}

type EmbeddingConfig struct {
	APIKey         string `mapstructure:"api_key"`
	BaseURL        string `mapstructure:"base_url"`
	Model          string `mapstructure:"model"`
	Dimensions     int    `mapstructure:"dimensions"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}

type PromptsConfig struct {
	Extract        string `mapstructure:"extract"`
	Match          string `mapstructure:"match"`
	Prefill        string `mapstructure:"prefill"`
	Summarize      string `mapstructure:"summarize"`
	Search         string `mapstructure:"search"`
	SearchAnalysis string `mapstructure:"search_analysis"`
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
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetDefault("filematch.weight_jaro", 0.4)
	v.SetDefault("filematch.weight_keyword", 0.4)
	v.SetDefault("filematch.weight_prefix", 0.2)
	v.SetDefault("filematch.threshold", 0.6)
	v.SetDefault("filematch.stop_words", []string{"复印件", "原件", "扫描件", "照片", "图片", "副本", "电子版", "扫描"})
	v.SetDefault("search.method", "structured")
	v.SetDefault("search.max_results", 10)
	v.SetDefault("search.vector.top_k", 20)
	v.SetDefault("search.vector.min_score", 0.7)
	v.SetDefault("search.vector.max_analysis", 5)
	v.SetDefault("search.vector.rerank", true)
	v.SetDefault("search.vector.mqe.enabled", true)
	v.SetDefault("search.vector.mqe.n_queries", 3)
	v.SetDefault("search.vector.mqe.rrf_k", 60.0)
	v.SetDefault("search.vector.hyde.enabled", false)
	v.SetDefault("search.vector.hyde.max_tokens", 512)

	v.SetDefault("file_parser.enabled", true)
	v.SetDefault("file_parser.venv_path", "sidecar/file-parser/venv")
	v.SetDefault("file_parser.script_path", "sidecar/file-parser/server.py")
	v.SetDefault("file_parser.timeout_sec", 30)

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
	MustLoadPromptFile("config/prompts/search.txt", &cfg.AI.Prompts.Search)
	MustLoadPromptFile("config/prompts/search_analysis.txt", &cfg.AI.Prompts.SearchAnalysis)
	return &cfg, nil
}
