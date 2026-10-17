package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// FileMode is the permission of config.toml. The file can hold API keys in
// plaintext (like ~/.aws/credentials or ~/.netrc), so only the owner may read
// it. Environment variables (INODE_LLM_API_KEY, ...) avoid storing keys at all.
const FileMode fs.FileMode = 0o600

// FileName is the config file inside the config directory.
const FileName = "config.toml"

// Set writes key = value into <dir>/config.toml, keeping the other keys, and
// leaves the file at FileMode. An existing file with looser permissions (as
// written by inode before v0.5.0) is tightened. The directory is created with
// mode 0700 if missing.
func Set(dir, key string, value any) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, FileName)

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	v.SetConfigPermissions(FileMode)
	if _, err := os.Stat(path); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
	}
	v.Set(key, value)

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	// WriteConfigAs applies FileMode only when it creates the file.
	return os.Chmod(path, FileMode)
}

// LooseMode reports the permissions of <dir>/config.toml when group or other
// users can read or write it. ok is false when the file is private or absent.
func LooseMode(dir string) (mode fs.FileMode, ok bool) {
	fi, err := os.Stat(filepath.Join(dir, FileName))
	if err != nil {
		return 0, false
	}
	perm := fi.Mode().Perm()
	return perm, perm&0o077 != 0
}
