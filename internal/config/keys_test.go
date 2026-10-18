package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// Regression for #81: `inode config set` kept its own allowlist, which drifted
// from Config (db.backend, db.dsn, search.*, mcp.reveal_sensitive were missing).
func TestKeysMatchConfigFields(t *testing.T) {
	var fields []string
	walk(reflect.ValueOf(&Config{}).Elem(), "", func(name string, _ reflect.Value) {
		fields = append(fields, name)
	})

	var keys []string
	for _, k := range Keys {
		keys = append(keys, k.Name)
	}
	slices.Sort(fields)
	slices.Sort(keys)
	if !slices.Equal(fields, keys) {
		t.Errorf("Keys and Config fields differ\n  Config: %v\n  Keys:   %v", fields, keys)
	}
}

func TestKeyKindsMatchConfigFieldTypes(t *testing.T) {
	kinds := map[reflect.Kind]Kind{
		reflect.String: String, reflect.Bool: Bool, reflect.Int: Int, reflect.Float32: Float,
	}
	walk(reflect.ValueOf(&Config{}).Elem(), "", func(name string, v reflect.Value) {
		k, _ := LookupKey(name)
		if want, ok := kinds[v.Kind()]; !ok || k.Kind != want {
			t.Errorf("%s: Key kind %v does not match field type %s", name, k.Kind, v.Kind())
		}
	})
}

func TestKeyParse(t *testing.T) {
	tests := []struct {
		key, raw string
		want     any
		wantErr  string
	}{
		{"db.backend", "postgres", "postgres", ""},
		{"db.dsn", "postgres://u:p@localhost:5432/db", "postgres://u:p@localhost:5432/db", ""},
		{"mcp.reveal_sensitive", "true", true, ""},
		{"search.top_k", "10", 10, ""},
		{"search.max_distance", "0.8", 0.8, ""},
		{"embedding.dimension", "1024", 1024, ""},
		{"search.top_k", "abc", nil, "search.top_k must be an integer"},
		{"mcp.reveal_sensitive", "yes", nil, "must be true or false"},
		{"search.max_distance", "far", nil, "must be a number"},
		{"db.backend", "mysql", nil, "must be one of sqlite, postgres"},
		{"llm.backend", "claude", nil, "must be one of ollama, claude-api, openai"},
	}
	for _, tt := range tests {
		k, ok := LookupKey(tt.key)
		if !ok {
			t.Fatalf("LookupKey(%q) not found", tt.key)
		}
		got, err := k.Parse(tt.raw)
		if tt.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Parse(%s, %q) error = %v, want %q", tt.key, tt.raw, err, tt.wantErr)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("Parse(%s, %q) = %v (%T), %v; want %v (%T)", tt.key, tt.raw, got, got, err, tt.want, tt.want)
		}
	}
	if _, ok := LookupKey("db.password"); ok {
		t.Error("LookupKey accepted an unknown key")
	}
}

// Every documented key can be set, is written with its type (top_k = 10, not
// "10"), and reads back into Config.
func TestSetEveryKeyRoundTrips(t *testing.T) {
	dir := t.TempDir()
	samples := map[string]string{
		"llm.backend": "openai", "llm.model": "gpt-4o-mini", "llm.base_url": "https://api.openai.com/v1",
		"llm.api_key": "sk-test", "embedding.backend": "voyage", "embedding.model": "voyage-3",
		"embedding.base_url": "http://localhost:11434", "embedding.api_key": "pa-test",
		"embedding.dimension": "1024", "db.backend": "postgres", "db.path": "/tmp/notes.db",
		"db.dsn": "postgres://u:p@localhost/db", "defaults.sensitive": "false", "search.top_k": "10",
		"search.max_distance": "0.8", "mcp.reveal_sensitive": "true", "log.level": "debug",
	}
	for _, k := range Keys {
		raw, ok := samples[k.Name]
		if !ok {
			t.Fatalf("no sample value for %s", k.Name)
		}
		v, err := k.Parse(raw)
		if err != nil {
			t.Fatalf("Parse %s: %v", k.Name, err)
		}
		if err := Set(dir, k.Name, v); err != nil {
			t.Fatalf("Set %s: %v", k.Name, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"top_k = 10", "reveal_sensitive = true", "dimension = 1024", "max_distance = 0.8"} {
		if !strings.Contains(string(data), line) {
			t.Errorf("config.toml does not contain %q:\n%s", line, data)
		}
	}

	v := viper.New()
	v.SetConfigFile(filepath.Join(dir, FileName))
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatal(err)
	}
	got := Values(&cfg)
	for name, raw := range samples {
		if s := fmt.Sprint(got[name]); s != raw {
			t.Errorf("%s read back as %q, want %q", name, s, raw)
		}
	}
}

// Before #81, config set wrote every value as a string (dimension = "1024").
// Those files must keep loading.
func TestStringValuesFromOlderVersionsStillLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	old := "[embedding]\ndimension = \"1024\"\n[defaults]\nsensitive = \"false\"\n"
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.Embedding.Dimension != 1024 || cfg.Defaults.Sensitive {
		t.Errorf("dimension = %d, sensitive = %v; want 1024, false", cfg.Embedding.Dimension, cfg.Defaults.Sensitive)
	}
}
