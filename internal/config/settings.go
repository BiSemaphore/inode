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
}

type EmbeddingConfig struct {
	Backend string `mapstructure:"backend"`
	Model   string `mapstructure:"model"`
	APIKey  string `mapstructure:"api_key"`
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
	_ = v.BindEnv("embedding.api_key", "INODE_EMBEDDING_API_KEY")
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

	// Defaults
	v.SetDefault("llm.backend", "claude-api")
	v.SetDefault("llm.model", "claude-sonnet-4-6")
	v.SetDefault("embedding.backend", "voyage")
	v.SetDefault("embedding.model", "voyage-3")
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
