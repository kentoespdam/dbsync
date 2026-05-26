package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/scrypt"
)

const (
	ScryptN      = 32768
	ScryptR      = 8
	ScryptP      = 1
	KeyLen       = 32
	NonceSize    = 12
	MinSaltSize  = 16
)

// DeriveKey derives a 32-byte key from a master password and salt using scrypt.
func DeriveKey(masterPassword string, salt []byte) ([]byte, error) {
	if len(salt) < MinSaltSize {
		return nil, fmt.Errorf("salt must be at least %d bytes", MinSaltSize)
	}
	return scrypt.Key([]byte(masterPassword), salt, ScryptN, ScryptR, ScryptP, KeyLen)
}

// Encrypt encrypts plaintext using AES-256-GCM with the provided 32-byte key.
// It returns a base64 encoded string containing the nonce and ciphertext.
func Encrypt(plaintext, key []byte) (string, error) {
	if len(key) != KeyLen {
		return "", fmt.Errorf("key must be exactly %d bytes", KeyLen)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Format: nonce || ciphertext
	combined := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt decrypts a base64 encoded string (nonce || ciphertext) using AES-256-GCM.
func Decrypt(b64 string, key []byte) ([]byte, error) {
	if len(key) != KeyLen {
		return nil, fmt.Errorf("key must be exactly %d bytes", KeyLen)
	}

	combined, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(combined) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := combined[:nonceSize], combined[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
