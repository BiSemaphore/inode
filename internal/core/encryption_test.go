package core

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := make([]byte, 32) // zeroed key — fine for test
	plaintext := []byte("sk_test_stripe_secret_key_xxxxx")
	noteID := []byte("note-uuid-1234")

	ciphertext, err := Encrypt(key, plaintext, noteID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext must differ from plaintext")
	}

	recovered, err := Decrypt(key, ciphertext, noteID)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(recovered, plaintext) {
		t.Fatalf("expected %q, got %q", plaintext, recovered)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key := make([]byte, 32)
	wrongKey := make([]byte, 32)
	wrongKey[0] = 0xFF

	ciphertext, _ := Encrypt(key, []byte("secret"), []byte("id"))
	_, err := Decrypt(wrongKey, ciphertext, []byte("id"))
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestDecryptWrongNoteID(t *testing.T) {
	key := make([]byte, 32)
	ciphertext, _ := Encrypt(key, []byte("secret"), []byte("note-id-A"))

	_, err := Decrypt(key, ciphertext, []byte("note-id-B"))
	if err == nil {
		t.Fatal("expected error when decrypting with wrong noteID (AAD mismatch)")
	}
}

func TestEncryptProducesUniqueIVs(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("same content")
	noteID := []byte("same-id")

	ct1, _ := Encrypt(key, plaintext, noteID)
	ct2, _ := Encrypt(key, plaintext, noteID)

	if bytes.Equal(ct1, ct2) {
		t.Fatal("two encryptions of the same content must produce different ciphertext (unique IV)")
	}
}

func TestDecryptTamperedData(t *testing.T) {
	key := make([]byte, 32)
	ciphertext, _ := Encrypt(key, []byte("secret"), []byte("id"))

	// Flip a bit in the ciphertext body
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err := Decrypt(key, ciphertext, []byte("id"))
	if err == nil {
		t.Fatal("expected error when decrypting tampered data")
	}
}
