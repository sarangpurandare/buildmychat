package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// User represents a user in the database.
type User struct {
	ID             uuid.UUID `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	Email          string    `db:"email"`
	HashedPassword string    `db:"hashed_password"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	// Add other fields as needed (e.g., Name, Role, LastLoginAt)
}

// Organization represents an organization or workspace in the database.
type Organization struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	// Add other fields as needed (e.g., SubscriptionStatus, OwnerUserID)
}

// IntegrationCredential represents stored credentials for external services.
type IntegrationCredential struct {
	ID                   uuid.UUID   `db:"id"`
	OrganizationID       uuid.UUID   `db:"organization_id"`
	ServiceType          ServiceType `db:"service_type"` // Use the ServiceType defined in models/api.go
	CredentialName       string      `db:"credential_name"`
	EncryptedCredentials []byte      `db:"encrypted_credentials"` // Holds raw DECODED (from base64 JSON) bytes from DB
	Status               string      `db:"status"`
	CreatedAt            time.Time   `db:"created_at"`
	UpdatedAt            time.Time   `db:"updated_at"`
}

// KnowledgeBase represents a configured knowledge base instance.
type KnowledgeBase struct {
	ID             uuid.UUID       `db:"id"`
	OrganizationID uuid.UUID       `db:"organization_id"`
	CredentialID   uuid.UUID       `db:"credential_id"`
	ServiceType    ServiceType     `db:"service_type"` // NOTION
	Name           string          `db:"name"`
	Configuration  json.RawMessage `db:"configuration"` // Stored as JSONB
	IsActive       bool            `db:"is_active"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

// Interface represents a configured chat interface instance.
type Interface struct {
	ID             uuid.UUID       `db:"id"`
	OrganizationID uuid.UUID       `db:"organization_id"`
	CredentialID   uuid.UUID       `db:"credential_id"`
	ServiceType    ServiceType     `db:"service_type"` // SLACK
	Name           string          `db:"name"`
	Configuration  json.RawMessage `db:"configuration"` // Stored as JSONB
	IsActive       bool            `db:"is_active"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

// Chatbot represents the central hub entity.
// Adding this for reference as it's used in mapping tables.
type Chatbot struct {
	ID             uuid.UUID       `db:"id"`
	OrganizationID uuid.UUID       `db:"organization_id"`
	Name           string          `db:"name"`
	SystemPrompt   *string         `db:"system_prompt"` // Use pointer for nullable text
	IsActive       bool            `db:"is_active"`
	ChatCount      int64           `db:"chat_count"`
	LLMModel       *string         `db:"llm_model"`     // Use pointer for nullable varchar
	Configuration  json.RawMessage `db:"configuration"` // Stored as JSONB
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}
