package config

import (
	"encoding/hex"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application configuration values loaded from environment variables.
type Config struct {
	DatabaseURL     string
	JWTSecret       string
	HTTPPort        string
	TokenExpiration time.Duration
	EncryptionKey   []byte // Raw key bytes (32 for AES-256)
	// Add other config fields like OpenAIKey, SlackToken, NotionKey, etc.
}

// LoadConfig loads configuration from environment variables.
// It looks for a .env file first, then checks actual environment variables.
func LoadConfig() (*Config, error) {
	// Attempt to load .env file (useful for development)
	err := godotenv.Load() // Loads .env from the current directory or parent dirs
	if err != nil {
		log.Println("Warning: Could not load .env file. Using environment variables only.", err)
		// Don't fail if .env is not present, might be in production
	}

	port := getEnv("HTTP_PORT", "8080")
	jwtSecret := getEnv("JWT_SECRET", "default-super-secret-key") // CHANGE THIS IN PRODUCTION!
	dbURL := getEnv("DATABASE_URL", "")                           // No default, should fail if not set
	log.Println("dbURL", dbURL)
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set.")
	}

	tokenExpStr := getEnv("JWT_EXPIRATION_HOURS", "24") // Default 24 hours
	tokenExpHours, err := strconv.Atoi(tokenExpStr)
	if err != nil {
		log.Printf("Warning: Invalid JWT_EXPIRATION_HOURS '%s', using default 24h. Error: %v", tokenExpStr, err)
		tokenExpHours = 24
	}

	// Load and decode the Encryption Key (MUST be 64 hex characters for 32 bytes)
	encryptionKeyHex := getEnv("ENCRYPTION_KEY", "")
	if encryptionKeyHex == "" {
		log.Fatal("FATAL: ENCRYPTION_KEY environment variable is not set.")
	}
	encryptionKeyBytes, err := hex.DecodeString(encryptionKeyHex)
	if err != nil {
		log.Fatalf("FATAL: Failed to decode ENCRYPTION_KEY from hex: %v", err)
	}
	if len(encryptionKeyBytes) != 32 {
		log.Fatalf("FATAL: ENCRYPTION_KEY must be 32 bytes (64 hex characters) long, got %d bytes", len(encryptionKeyBytes))
	}

	cfg := &Config{
		HTTPPort:        port,
		JWTSecret:       jwtSecret,
		DatabaseURL:     dbURL,
		TokenExpiration: time.Hour * time.Duration(tokenExpHours),
		EncryptionKey:   encryptionKeyBytes,
	}

	log.Printf("Loaded config: Port=%s, DB_URL=***, TokenExp=%s, EncryptionKey=***", cfg.HTTPPort, cfg.TokenExpiration)

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	log.Printf("Env variable %s not set, using default: %s", key, fallback)
	return fallback
}
