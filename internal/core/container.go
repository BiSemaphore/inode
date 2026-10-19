package core

import (
	"context"
	"fmt"
	"os"

	"github.com/BiSemaphore/inode/internal/adapters/db"
	"github.com/BiSemaphore/inode/internal/adapters/embedding"
	"github.com/BiSemaphore/inode/internal/adapters/llm"
	"github.com/BiSemaphore/inode/internal/config"
)

// Container holds all initialised services and adapters.
type Container struct {
	Notes  *NoteService
	Search *SearchService
	DB     db.Adapter
}

// NewContainer initialises every dependency from the loaded config.
func NewContainer(cfg *config.Config) (*Container, error) {
	cfg, err := ResolveBackends(cfg)
	if err != nil {
		return nil, err
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

	// DB adapter — selected by backend config.
	var dbAdapter db.Adapter
	switch cfg.DB.Backend {
	case "postgres":
		dbAdapter, err = db.NewPostgresAdapter(context.Background(), cfg.DB.DSN, cfg.Embedding.Dimension)
		if err != nil {
			return nil, fmt.Errorf("init postgres: %w", err)
		}
	case "sqlite", "":
		dbPath := cfg.DB.Path
		if dbPath == "" {
			dbPath = configDir + "/notes.db"
		}
		if len(dbPath) >= 2 && dbPath[:2] == "~/" {
			home, _ := os.UserHomeDir()
			dbPath = home + dbPath[1:]
		}
		dbAdapter, err = db.NewSQLiteAdapter(dbPath, cfg.Embedding.Dimension)
		if err != nil {
			return nil, fmt.Errorf("init sqlite: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown db.backend %q (supported: sqlite, postgres)", cfg.DB.Backend)
	}

	// LLM adapter — selected by backend config.
	var llmAdapter llm.Adapter
	switch cfg.LLM.Backend {
	case "claude-api":
		if cfg.LLM.APIKey == "" {
			return nil, fmt.Errorf("llm.api_key required for claude-api backend — run: inode config set llm.api_key <key>")
		}
		llmAdapter = llm.NewClaudeAdapter(cfg.LLM.APIKey, cfg.LLM.Model)
	case "openai":
		if cfg.LLM.APIKey == "" {
			return nil, fmt.Errorf("llm.api_key required for openai backend; run: inode config set llm.api_key <key>")
		}
		llmAdapter = llm.NewOpenAIAdapter(cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.BaseURL)
	case "ollama", "":
		llmAdapter = llm.NewOllamaAdapter(cfg.LLM.BaseURL, cfg.LLM.Model)
	default:
		return nil, fmt.Errorf("unknown llm.backend %q (supported: ollama, claude-api, openai)", cfg.LLM.Backend)
	}

	// Embedding adapter — selected by backend config.
	var embAdapter embedding.Adapter
	switch cfg.Embedding.Backend {
	case "voyage":
		if cfg.Embedding.APIKey == "" {
			return nil, fmt.Errorf("embedding.api_key required for voyage backend — run: inode config set embedding.api_key <key>")
		}
		embAdapter = embedding.NewVoyageAdapter(cfg.Embedding.APIKey, cfg.Embedding.Model)
	case "ollama", "":
		embAdapter = embedding.NewOllamaEmbeddingAdapter(cfg.Embedding.BaseURL, cfg.Embedding.Model)
	default:
		return nil, fmt.Errorf("unknown embedding.backend %q (supported: ollama, voyage)", cfg.Embedding.Backend)
	}

	// Core services.
	tagger := NewTaggerService(llmAdapter)
	notes := NewNoteService(dbAdapter, embAdapter, tagger, keyMgr)
	search := NewSearchService(dbAdapter, embAdapter, llmAdapter, keyMgr, cfg.Search.MaxDistance, cfg.Search.TopK)
	if err := guardEmbeddingSpace(context.Background(), dbAdapter, configuredSpace(cfg), notes, search); err != nil {
		_ = dbAdapter.Close()
		return nil, fmt.Errorf("check embedding model: %w", err)
	}

	// One-time check of summaries saved before they were validated (#53).
	// After the first run this is a single query that returns nothing. A
	// failure must not block normal use, so it is reported and skipped.
	if n, err := notes.ScrubSensitiveSummaries(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not check stored summaries of sensitive notes: %v\n", err)
	} else if n > 0 {
		fmt.Fprintf(os.Stderr, "inode: replaced %d stored summaries of sensitive notes that contained secret values\n", n)
	}

	return &Container{
		Notes:  notes,
		Search: search,
		DB:     dbAdapter,
	}, nil
}
