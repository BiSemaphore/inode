package config

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Kind is the type of a config value, used to parse `inode config set` input.
type Kind int

const (
	String Kind = iota
	Bool
	Int
	Float
)

func (k Kind) String() string {
	switch k {
	case Bool:
		return "true or false"
	case Int:
		return "an integer"
	case Float:
		return "a number"
	default:
		return "a string"
	}
}

// Key describes one setting in config.toml. Keys is the single list that
// `inode config set` validates against and `inode config show` prints; a test
// checks it matches the fields of Config.
type Key struct {
	Name    string
	Kind    Kind
	Allowed []string // non-empty: the only accepted values
	Secret  bool     // redacted by `inode config show`
	Help    string
}

// Keys lists every setting, in the order `inode config show` prints them.
var Keys = []Key{
	{Name: "llm.backend", Allowed: []string{"ollama", "claude-api", "openai"}, Help: "LLM for tagging, summaries and answers"},
	{Name: "llm.model", Help: "e.g. llama3.2, claude-sonnet-5-5, gpt-4o-mini"},
	{Name: "llm.base_url", Help: "Ollama URL, or an OpenAI-compatible endpoint"},
	{Name: "llm.api_key", Secret: true, Help: "Anthropic or OpenAI API key (or set INODE_LLM_API_KEY)"},
	{Name: "embedding.backend", Allowed: []string{"ollama", "voyage"}, Help: "embedding provider"},
	{Name: "embedding.model", Help: "e.g. nomic-embed-text, voyage-3"},
	{Name: "embedding.base_url", Help: "Ollama URL for embeddings"},
	{Name: "embedding.api_key", Secret: true, Help: "Voyage AI API key (or set INODE_EMBEDDING_API_KEY)"},
	{Name: "embedding.dimension", Kind: Int, Help: "vector size of the embedding model"},
	{Name: "db.backend", Allowed: []string{"sqlite", "postgres"}, Help: "storage engine"},
	{Name: "db.path", Help: "SQLite database file"},
	{Name: "db.dsn", Secret: true, Help: "Postgres connection string"},
	{Name: "defaults.sensitive", Kind: Bool, Help: "treat new items as sensitive when unsure"},
	{Name: "search.top_k", Kind: Int, Help: "candidates fetched per search"},
	{Name: "search.max_distance", Kind: Float, Help: "drop matches farther than this (<0 disables)"},
	{Name: "mcp.reveal_sensitive", Kind: Bool, Help: "let MCP clients read sensitive items"},
	{Name: "log.level", Allowed: []string{"debug", "info", "warn", "error"}, Help: "log verbosity (not read by any command yet)"},
}

// LookupKey returns the Key with the given name.
func LookupKey(name string) (Key, bool) {
	i := slices.IndexFunc(Keys, func(k Key) bool { return k.Name == name })
	if i < 0 {
		return Key{}, false
	}
	return Keys[i], true
}

// Parse converts command-line input to the value written to config.toml,
// so that `search.top_k 10` is stored as the integer 10, not the string "10".
func (k Key) Parse(raw string) (any, error) {
	var v any
	var err error
	switch k.Kind {
	case Bool:
		v, err = strconv.ParseBool(raw)
	case Int:
		v, err = strconv.Atoi(raw)
	case Float:
		v, err = strconv.ParseFloat(raw, 64)
	default:
		v = raw
	}
	if err != nil {
		return nil, fmt.Errorf("%s must be %s, got %q", k.Name, k.Kind, raw)
	}
	if len(k.Allowed) > 0 && !slices.Contains(k.Allowed, raw) {
		return nil, fmt.Errorf("%s must be one of %s, got %q", k.Name, strings.Join(k.Allowed, ", "), raw)
	}
	return v, nil
}

// Values returns the value of every key in cfg, by key name.
func Values(cfg *Config) map[string]any {
	out := map[string]any{}
	walk(reflect.ValueOf(cfg).Elem(), "", func(name string, v reflect.Value) {
		out[name] = v.Interface()
	})
	return out
}

// walk calls fn for every leaf field of a config struct, named by its
// mapstructure tags joined with dots (the key used in config.toml).
func walk(v reflect.Value, prefix string, fn func(name string, v reflect.Value)) {
	t := v.Type()
	for i := range t.NumField() {
		name := prefix + t.Field(i).Tag.Get("mapstructure")
		if f := v.Field(i); f.Kind() == reflect.Struct {
			walk(f, name+".", fn)
		} else {
			fn(name, f)
		}
	}
}
