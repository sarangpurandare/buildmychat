package auth

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword generates a bcrypt hash for the given password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error generating bcrypt hash: %v", err)
		return "", err
	}
	return string(bytes), nil
}

// CheckPasswordHash compares a plaintext password with a stored bcrypt hash.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if err != bcrypt.ErrMismatchedHashAndPassword {
			// Log unexpected errors, but still return false for security
			log.Printf("Error comparing password hash: %v", err)
		}
		return false // Passwords don't match or error occurred
	}
	return true // Passwords match
}
