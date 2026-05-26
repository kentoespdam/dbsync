package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	password := "master-password"
	salt := make([]byte, 16)
	plaintext := []byte("secret-payload")

	key, err := DeriveKey(password, salt)
	if err != nil {
		t.Fatalf("DeriveKey failed: %v", err)
	}

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted content mismatch. Got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestKDFDeterminism(t *testing.T) {
	password := "password"
	salt := []byte("salt-must-be-16-bytes-at-least")

	key1, _ := DeriveKey(password, salt)
	key2, _ := DeriveKey(password, salt)

	if !bytes.Equal(key1, key2) {
		t.Error("KDF not deterministic")
	}
}

func TestTamperDetection(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("hello")

	encrypted, _ := Encrypt(plaintext, key)
	decoded, _ := base64.StdEncoding.DecodeString(encrypted)
	
	// Flip a bit in the ciphertext part (not the nonce)
	decoded[len(decoded)-1] ^= 0x01
	tampered := base64.StdEncoding.EncodeToString(decoded)

	_, err := Decrypt(tampered, key)
	if err == nil {
		t.Error("Decrypt should have failed on tampered data")
	}
}

func TestNonceUniqueness(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("hello")
	seen := make(map[string]bool)

	for i := 0; i < 1000; i++ {
		encrypted, _ := Encrypt(plaintext, key)
		if seen[encrypted] {
			t.Fatalf("Duplicate ciphertext detected at iteration %d", i)
		}
		seen[encrypted] = true
	}
}

func TestKeyLengthValidation(t *testing.T) {
	shortKey := make([]byte, 16)
	plaintext := []byte("hello")

	_, err := Encrypt(plaintext, shortKey)
	if err == nil {
		t.Error("Encrypt should fail with short key")
	}

	_, err = Decrypt("YWFhYWFhYWFhYWFhYWFhYWFhYWE=", shortKey)
	if err == nil {
		t.Error("Decrypt should fail with short key")
	}
}

func TestSaltLengthValidation(t *testing.T) {
	shortSalt := make([]byte, 8)
	_, err := DeriveKey("password", shortSalt)
	if err == nil {
		t.Error("DeriveKey should fail with short salt")
	}
}
