package core

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func mustDerive(t *testing.T, km *KeyManager) []byte {
	t.Helper()
	k, err := km.DeriveKey()
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	return k
}

func TestKeyManager_FirstRunCreatesSecretOnceAndKeyIsStable(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()

	km1, err := NewKeyManager(dir)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	stored, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		t.Fatalf("secret not stored in keychain: %v", err)
	}

	km2, err := NewKeyManager(dir)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if again, _ := keyring.Get(keyringService, keyringUser); again != stored {
		t.Error("second run replaced the master secret")
	}
	if !bytes.Equal(mustDerive(t, km1), mustDerive(t, km2)) {
		t.Error("derived key changed between runs")
	}
	if fileExists(filepath.Join(dir, ".key")) {
		t.Error("key file written although the keychain worked")
	}
}

func TestKeyManager_KeychainErrorAfterInitRefusesAndWritesNothing(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	if _, err := NewKeyManager(dir); err != nil {
		t.Fatal(err)
	}

	// The keychain becomes unreadable: locked, access denied, or a HOME the
	// OS keychain cannot resolve.
	keyring.MockInitWithError(errors.New("keychain locked"))
	_, err := NewKeyManager(dir)
	if err == nil {
		t.Fatal("expected an error, got a key manager")
	}
	if !strings.Contains(err.Error(), "refusing to create a new secret") || !strings.Contains(err.Error(), "keychain locked") {
		t.Errorf("error should explain the refusal and keep the cause, got: %v", err)
	}
	if fileExists(filepath.Join(dir, ".key")) {
		t.Error("a new secret was written to the key file")
	}
}

func TestKeyManager_SecretMissingButInitialisedRefuses(t *testing.T) {
	keyring.MockInit() // empty keychain: ErrNotFound
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".salt"), []byte(base64.StdEncoding.EncodeToString(make([]byte, 16))), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := NewKeyManager(dir); err == nil {
		t.Fatal("expected an error when .salt exists but no secret can be found")
	}
	if _, err := keyring.Get(keyringService, keyringUser); !errors.Is(err, keyring.ErrNotFound) {
		t.Errorf("a new secret was stored in the keychain (Get err = %v)", err)
	}
	if fileExists(filepath.Join(dir, ".key")) {
		t.Error("a new secret was written to the key file")
	}
}

func TestKeyManager_UnreachableKeychainOnFirstRunUsesKeyFile(t *testing.T) {
	keyring.MockInitWithError(errors.New("org.freedesktop.secrets not provided"))
	dir := t.TempDir()

	km, err := NewKeyManager(dir)
	if err != nil {
		t.Fatalf("headless first run: %v", err)
	}
	path := filepath.Join(dir, ".key")
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("key file not written: %v", err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("key file mode = %o, want 600", fi.Mode().Perm())
	}
	k1 := mustDerive(t, km)

	// Later runs on the same headless machine use the same file.
	km2, err := NewKeyManager(dir)
	if err != nil {
		t.Fatalf("headless second run: %v", err)
	}
	if !bytes.Equal(k1, mustDerive(t, km2)) {
		t.Error("derived key changed between headless runs")
	}
}

func TestKeyManager_ExistingKeyFileIsUsedNotReplaced(t *testing.T) {
	keyring.MockInit() // keychain empty, but a key file exists from a headless setup
	dir := t.TempDir()
	secret := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	if err := os.WriteFile(filepath.Join(dir, ".key"), []byte(secret), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".salt"), []byte(base64.StdEncoding.EncodeToString(make([]byte, 16))), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := NewKeyManager(dir); err != nil {
		t.Fatalf("existing key file should be enough: %v", err)
	}
	if _, err := keyring.Get(keyringService, keyringUser); !errors.Is(err, keyring.ErrNotFound) {
		t.Error("a new secret was stored in the keychain although a key file exists")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, ".key")); string(data) != secret {
		t.Error("key file contents changed")
	}
}
