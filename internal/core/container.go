package core

import (
	"fmt"
	"os"

	"github.com/shahid-io/inode/internal/adapters/db"
	"github.com/shahid-io/inode/internal/adapters/embedding"
	"github.com/shahid-io/inode/internal/adapters/llm"
	"github.com/shahid-io/inode/internal/config"
)

// Container holds all initialised services and adapters.
// It is built once at CLI startup and passed to commands.
type Container struct {
	Notes  *NoteService
	Search *SearchService
	DB     db.Adapter
}

// NewContainer initialises every dependency from the loaded config.
func NewContainer(cfg *config.Config) (*Container, error) {
	// Validate required keys.
	if cfg.LLM.APIKey == "" {
		return nil, fmt.Errorf("LLM API key not set — run: inode config set llm.api_key <key>")
	}
	if cfg.Embedding.APIKey == "" {
		return nil, fmt.Errorf("embedding API key not set — run: inode config set embedding.api_key <key>")
	}

	// Key manager (encryption).
	configDir, err := config.ConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve config dir: %w", err)
	}
	keyMgr, err := NewKeyManager(configDir)
	if err != nil {
		return nil, fmt.Errorf("init key manager: %w", err)
	}

	// DB adapter.
	dbPath := cfg.DB.Path
	if dbPath == "" {
		dbPath = configDir + "/notes.db"
	}
	// Expand ~ manually (os.UserHomeDir not needed; config already resolves it via viper).
	if len(dbPath) >= 2 && dbPath[:2] == "~/" {
		home, _ := os.UserHomeDir()
		dbPath = home + dbPath[1:]
	}
	dbAdapter, err := db.NewSQLiteAdapter(dbPath)
	if err != nil {
		return nil, fmt.Errorf("init db: %w", err)
	}

	// Adapters.
	embAdapter := embedding.NewVoyageAdapter(cfg.Embedding.APIKey, cfg.Embedding.Model)
	llmAdapter := llm.NewClaudeAdapter(cfg.LLM.APIKey, cfg.LLM.Model)

	// Core services.
	tagger := NewTaggerService(llmAdapter)
	notes := NewNoteService(dbAdapter, embAdapter, tagger, keyMgr)
	search := NewSearchService(dbAdapter, embAdapter, llmAdapter, keyMgr)

	return &Container{
		Notes:  notes,
		Search: search,
		DB:     dbAdapter,
	}, nil
}
