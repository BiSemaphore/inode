package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/argon2"
)

const (
	keyringService = "inode"
	keyringUser    = "master-secret"

	argonTime    = 2
	argonMemory  = 64 * 1024 // 64MB
	argonThreads = 1
	argonKeyLen  = 32
)

// KeyManager handles master secret storage and encryption key derivation.
// The derived key is never stored — it is rederived per process.
type KeyManager struct {
	configDir string
	salt      []byte
}

// NewKeyManager initialises the key manager, generating a master secret and
// salt on first run if they don't exist.
func NewKeyManager(configDir string) (*KeyManager, error) {
	km := &KeyManager{configDir: configDir}

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	if err := km.ensureSecret(); err != nil {
		return nil, fmt.Errorf("ensure master secret: %w", err)
	}

	salt, err := km.loadOrCreateSalt()
	if err != nil {
		return nil, fmt.Errorf("load salt: %w", err)
	}
	km.salt = salt

	return km, nil
}

// DeriveKey derives a 32-byte AES key from the master secret and salt.
func (km *KeyManager) DeriveKey() ([]byte, error) {
	secret, err := km.loadSecret()
	if err != nil {
		return nil, err
	}
	return argon2.IDKey(secret, km.salt, argonTime, argonMemory, argonThreads, argonKeyLen), nil
}

// ensureSecret creates and stores a master secret if one doesn't exist.
func (km *KeyManager) ensureSecret() error {
	_, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		return nil // already exists
	}

	// Generate new 32-byte secret
	secret := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, secret); err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(secret)

	if err := keyring.Set(keyringService, keyringUser, encoded); err != nil {
		// Keyring unavailable (headless Linux/WSL) — fall back to key file
		return km.writeKeyFile(encoded)
	}
	return nil
}

// loadSecret retrieves the master secret from keyring or fallback key file.
func (km *KeyManager) loadSecret() ([]byte, error) {
	encoded, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		// Try fallback key file
		encoded, err = km.readKeyFile()
		if err != nil {
			return nil, fmt.Errorf("master secret not found in keyring or key file: %w", err)
		}
	}
	return base64.StdEncoding.DecodeString(encoded)
}

func (km *KeyManager) loadOrCreateSalt() ([]byte, error) {
	saltPath := filepath.Join(km.configDir, ".salt")
	data, err := os.ReadFile(saltPath)
	if err == nil {
		decoded, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			return nil, err
		}
		return decoded, nil
	}

	// Generate new salt
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(salt)
	if err := os.WriteFile(saltPath, []byte(encoded), 0600); err != nil {
		return nil, err
	}
	return salt, nil
}

func (km *KeyManager) writeKeyFile(encoded string) error {
	path := filepath.Join(km.configDir, ".key")
	fmt.Fprintf(os.Stderr, "warning: keyring unavailable, storing key in %s (chmod 600)\n", path)
	return os.WriteFile(path, []byte(encoded), 0600)
}

func (km *KeyManager) readKeyFile() (string, error) {
	path := filepath.Join(km.configDir, ".key")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// noteID is used as additional authenticated data — binds ciphertext to the note.
// Returns: iv[12] + sealed[N] (sealed includes ciphertext + 16-byte GCM auth tag).
func Encrypt(key, plaintext, noteID []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	iv := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	sealed := gcm.Seal(nil, iv, plaintext, noteID)

	result := make([]byte, len(iv)+len(sealed))
	copy(result, iv)
	copy(result[len(iv):], sealed)
	return result, nil
}

// Decrypt decrypts data produced by Encrypt.
// Returns an error if the auth tag fails — data has been tampered with.
func Decrypt(key, data, noteID []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	iv, sealed := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, iv, sealed, noteID)
}
