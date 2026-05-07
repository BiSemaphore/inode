package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	LLM       LLMConfig       `mapstructure:"llm"`
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	DB        DBConfig        `mapstructure:"db"`
	Defaults  DefaultsConfig  `mapstructure:"defaults"`
	Log       LogConfig       `mapstructure:"log"`
}

type LLMConfig struct {
	Backend string `mapstructure:"backend"`
	Model   string `mapstructure:"model"`
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

type EmbeddingConfig struct {
	Backend   string `mapstructure:"backend"`
	Model     string `mapstructure:"model"`
	APIKey    string `mapstructure:"api_key"`
	BaseURL   string `mapstructure:"base_url"`
	Dimension int    `mapstructure:"dimension"`
}

type DBConfig struct {
	Path string `mapstructure:"path"`
}

type DefaultsConfig struct {
	Sensitive bool `mapstructure:"sensitive"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

// Load reads config from (in priority order):
//  1. CLI flags (handled by cobra, passed in via overrides)
//  2. Environment variables (INODE_* prefix)
//  3. .env file (development only — loaded if present)
//  4. ~/.inode/config.toml
//  5. Hardcoded defaults
func Load() (*Config, error) {
	// Load .env if present — silently ignored if missing
	_ = godotenv.Load()

	v := viper.New()

	// Map INODE_LLM_API_KEY → llm.api_key etc.
	v.SetEnvPrefix("INODE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit bindings for keys that contain underscores
	_ = v.BindEnv("llm.api_key", "INODE_LLM_API_KEY")
	_ = v.BindEnv("llm.base_url", "INODE_LLM_BASE_URL")
	_ = v.BindEnv("embedding.api_key", "INODE_EMBEDDING_API_KEY")
	_ = v.BindEnv("embedding.base_url", "INODE_EMBEDDING_BASE_URL")
	_ = v.BindEnv("embedding.dimension", "INODE_EMBEDDING_DIMENSION")
	_ = v.BindEnv("log.level", "INODE_LOG_LEVEL")

	// Config file: ~/.inode/config.toml
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	configDir := filepath.Join(home, ".inode")
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(configDir)
	_ = v.ReadInConfig() // non-fatal if file doesn't exist yet

	// Defaults — Ollama local by default, no API keys required
	v.SetDefault("llm.backend", "ollama")
	v.SetDefault("llm.model", "llama3.2")
	v.SetDefault("llm.base_url", "http://localhost:11434")
	v.SetDefault("embedding.backend", "ollama")
	v.SetDefault("embedding.model", "nomic-embed-text")
	v.SetDefault("embedding.base_url", "http://localhost:11434")
	v.SetDefault("embedding.dimension", 768)
	v.SetDefault("db.path", filepath.Join(configDir, "notes.db"))
	v.SetDefault("defaults.sensitive", true)
	v.SetDefault("log.level", "info")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ConfigDir returns the path to the inode config directory (~/.inode).
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".inode"), nil
}
