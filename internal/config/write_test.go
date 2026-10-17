package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSetCreatesPrivateFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes")
	}
	dir := filepath.Join(t.TempDir(), ".inode")

	if err := Set(dir, "llm.api_key", "sk-test-123"); err != nil {
		t.Fatal(err)
	}

	assertMode(t, filepath.Join(dir, FileName), FileMode)
	if _, loose := LooseMode(dir); loose {
		t.Error("LooseMode reports a fresh config as loose")
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %o, want 700", fi.Mode().Perm())
	}
}

func TestSetTightensExistingFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	// A config written by inode before v0.5.0.
	if err := os.WriteFile(path, []byte("[llm]\nbackend = 'ollama'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil { // umask may have narrowed it
		t.Fatal(err)
	}
	if mode, loose := LooseMode(dir); !loose || mode != 0o644 {
		t.Fatalf("LooseMode = %o, %v; want 644, true", mode, loose)
	}

	if err := Set(dir, "llm.model", "llama3.2"); err != nil {
		t.Fatal(err)
	}

	assertMode(t, path, FileMode)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The existing key survives the write.
	if !strings.Contains(string(data), "ollama") || !strings.Contains(string(data), "llama3.2") {
		t.Errorf("config lost a key:\n%s", data)
	}
}

func TestLooseModeAbsentFile(t *testing.T) {
	if _, loose := LooseMode(t.TempDir()); loose {
		t.Error("LooseMode reports a missing file as loose")
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != want {
		t.Errorf("%s mode = %o, want %o", filepath.Base(path), got, want)
	}
}
