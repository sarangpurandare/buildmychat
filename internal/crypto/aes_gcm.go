package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidKeySize       = errors.New("invalid AES key size (must be 16, 24, or 32 bytes)")
	ErrInvalidCiphertext    = errors.New("ciphertext too short to contain nonce")
	ErrAuthenticationFailed = errors.New("ciphertext authentication failed")
)

// NewAESGCM creates a new AES-GCM cipher block based on the key size.
func NewAESGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		// aes.NewCipher checks key size (16, 24, 32 bytes)
		return nil, fmt.Errorf("%w: %v", ErrInvalidKeySize, err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		// This error is less likely if NewCipher succeeded but check anyway
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return aead, nil
}

// Encrypt encrypts plaintext using AES-GCM.
// It generates a random nonce and prepends it to the returned ciphertext.
func Encrypt(aead cipher.AEAD, plaintext []byte) ([]byte, error) {
	// Never use more than 2^32 random nonces with a given key because of the risk of repeat.
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal encrypts the plaintext and appends the authentication tag.
	// The nonce is passed explicitly and is not included in the output of Seal.
	// We prepend the nonce to the ciphertext manually for storage.
	ciphertext := aead.Seal(nil, nonce, plaintext, nil) // Pass nil for additional authenticated data

	// Prepend nonce to ciphertext
	ciphertextWithNonce := append(nonce, ciphertext...)

	return ciphertextWithNonce, nil
}

// Decrypt decrypts ciphertextWithNonce (which includes the prepended nonce) using AES-GCM.
func Decrypt(aead cipher.AEAD, ciphertextWithNonce []byte) ([]byte, error) {
	nonceSize := aead.NonceSize()
	if len(ciphertextWithNonce) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	// Extract the nonce and the actual ciphertext
	nonce := ciphertextWithNonce[:nonceSize]
	ciphertext := ciphertextWithNonce[nonceSize:]

	// Open decrypts the ciphertext, verifies the authentication tag, and returns the plaintext.
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil) // Pass nil for additional authenticated data
	if err != nil {
		// Common error here is "cipher: message authentication failed"
		return nil, fmt.Errorf("%w: %v", ErrAuthenticationFailed, err)
	}

	return plaintext, nil
}
