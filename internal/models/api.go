package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Request Structs ---

// SignupRequest defines the expected body for the signup endpoint.
type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	// Add other fields like Name if needed
}

// LoginRequest defines the expected body for the login endpoint.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// --- Response Structs ---

// UserResponse defines the user information returned by the API.
// Avoid returning sensitive info like HashedPassword.
type UserResponse struct {
	ID             uuid.UUID `json:"id"`
	Email          string    `json:"email"`
	OrganizationID uuid.UUID `json:"organization_id"`
	// Add Name, CreatedAt etc. if needed by the frontend
}

// AuthResponse defines the response body for successful authentication.
type AuthResponse struct {
	AccessToken string       `json:"access_token"`
	User        UserResponse `json:"user"`
}

// ErrorResponse defines the standard structure for API errors.
type ErrorResponse struct {
	Error string `json:"error"`
}

// --- Integration Credentials DTOs ---

// ServiceType defines the types of external services we can integrate with.
type ServiceType string

const (
	ServiceTypeNotion ServiceType = "NOTION"
	ServiceTypeSlack  ServiceType = "SLACK"
	// Add other service types here
)

// CreateCredentialRequest defines the body for creating a new integration credential.
// The Credentials map contains the raw secrets and is ONLY used for this request.
// It should NEVER be stored directly or returned in responses.
type CreateCredentialRequest struct {
	ServiceType    ServiceType       `json:"service_type"` // e.g., "NOTION", "SLACK"
	CredentialName *string           `json:"credential_name,omitempty"`
	Credentials    map[string]string `json:"credentials"`
}

// CredentialResponse defines the data returned when fetching integration credentials.
// It EXCLUDES the actual encrypted or raw secrets.
type CredentialResponse struct {
	ID             uuid.UUID   `json:"id"`
	OrganizationID uuid.UUID   `json:"organization_id"`
	ServiceType    ServiceType `json:"service_type"`
	CredentialName string      `json:"credential_name"`
	Status         string      `json:"status"` // e.g., "ACTIVE", "INVALID"
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// TestCredentialResponse defines the response for testing a credential's validity.
type TestCredentialResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"` // Optional message (e.g., error details on failure)
}

// --- Knowledge Base DTOs ---

// CreateKnowledgeBaseRequest defines the body for creating a knowledge base.
type CreateKnowledgeBaseRequest struct {
	Name          string          `json:"name"`
	CredentialID  uuid.UUID       `json:"credential_id"`           // Must be a credential of type NOTION
	Configuration json.RawMessage `json:"configuration,omitempty"` // Service-specific config (e.g., {"object_ids": [...]})
}

// KnowledgeBaseResponse defines the data returned for a knowledge base.
type KnowledgeBaseResponse struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	CredentialID   uuid.UUID       `json:"credential_id"`
	ServiceType    ServiceType     `json:"service_type"` // Should always be NOTION for now
	Name           string          `json:"name"`
	Configuration  json.RawMessage `json:"configuration,omitempty"`
	IsActive       bool            `json:"is_active"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// --- Interface DTOs ---

// CreateInterfaceRequest defines the body for creating an interface.
type CreateInterfaceRequest struct {
	Name          string          `json:"name"`
	CredentialID  uuid.UUID       `json:"credential_id"`           // Must be a credential of type SLACK
	Configuration json.RawMessage `json:"configuration,omitempty"` // Service-specific config (e.g., {"slack_team_id": "T123"})
}

// InterfaceResponse defines the data returned for an interface.
type InterfaceResponse struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	CredentialID   uuid.UUID       `json:"credential_id"`
	ServiceType    ServiceType     `json:"service_type"` // Should always be SLACK for now
	Name           string          `json:"name"`
	Configuration  json.RawMessage `json:"configuration,omitempty"`
	IsActive       bool            `json:"is_active"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// --- Chatbot DTOs ---

// CreateChatbotRequest defines the payload for creating a new chatbot.
// Name is optional initially, can be set later via update.
type CreateChatbotRequest struct {
	Name         *string `json:"name"` // Optional
	SystemPrompt *string `json:"system_prompt"`
	LLMModel     *string `json:"llm_model"`
	// Configuration can be set during creation if needed, but often set later.
	Configuration *json.RawMessage `json:"configuration"`
}

// ChatbotResponse defines the standard representation of a chatbot in API responses.
type ChatbotResponse struct {
	ID             uuid.UUID        `json:"id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	Name           string           `json:"name"` // Note: Name might be empty initially if not provided on create
	SystemPrompt   *string          `json:"system_prompt"`
	IsActive       bool             `json:"is_active"`
	ChatCount      int64            `json:"chat_count"`
	LLMModel       *string          `json:"llm_model"`
	Configuration  *json.RawMessage `json:"configuration"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	// Nested connected resources
	KnowledgeBases []KnowledgeBaseResponse `json:"knowledge_bases,omitempty"`
	Interfaces     []InterfaceResponse     `json:"interfaces,omitempty"`
}

// UpdateChatbotRequest defines the payload for updating an existing chatbot.
// Only fields present in the request will be updated.
type UpdateChatbotRequest struct {
	Name          *string          `json:"name"`
	SystemPrompt  *string          `json:"system_prompt"`
	LLMModel      *string          `json:"llm_model"`
	Configuration *json.RawMessage `json:"configuration"` // Allows updating parts or all of config
}

// UpdateChatbotStatusRequest defines the payload for activating/deactivating a chatbot.
type UpdateChatbotStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// ListChatbotsResponse defines the response structure for listing chatbots.
type ListChatbotsResponse struct {
	Chatbots []ChatbotResponse `json:"chatbots"`
}

// --- Chatbot Mapping DTOs ---

// AddKnowledgeBaseRequest defines the request for adding a KB to a chatbot
type AddKnowledgeBaseRequest struct {
	KBID uuid.UUID `json:"kb_id"`
}

// AddInterfaceRequest defines the request for adding an interface to a chatbot
type AddInterfaceRequest struct {
	InterfaceID uuid.UUID `json:"interface_id"`
}

// ChatbotMappingsResponse defines the response structure for listing chatbot connections
type ChatbotMappingsResponse struct {
	KnowledgeBases []KnowledgeBaseResponse `json:"knowledge_bases,omitempty"`
	Interfaces     []InterfaceResponse     `json:"interfaces,omitempty"`
}

// --- Chat DTOs ---

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role      string           `json:"role"`               // "user", "assistant", "system"
	Content   string           `json:"content"`            // The message text
	Timestamp int64            `json:"timestamp"`          // Unix timestamp (seconds since epoch)
	SentBy    string           `json:"sent_by"`            // "user", "assistant", "system" (duplicates Role for compatibility)
	Hide      int              `json:"hide"`               // 0 = show, 1 = hide
	Metadata  *json.RawMessage `json:"metadata,omitempty"` // Optional additional data
	// Media     []MediaAttachment `json:"media,omitempty"`    // Reverted: Optional media attachments
}

// MediaAttachment represents a media file attached to a message
/* Reverted: MediaAttachment struct
type MediaAttachment struct {
	Type string `json:"type"` // e.g., "image", "video", "audio", "document"
	URL  string `json:"url"`  // URL to the media file
	// Additional fields could include: title, thumbnail_url, file_size, etc.
}
*/

// CreateChatRequest defines the payload for creating a new chat.
type CreateChatRequest struct {
	ChatbotID      uuid.UUID        `json:"chatbot_id"`
	InterfaceID    *uuid.UUID       `json:"interface_id,omitempty"`     // Optional, for API-created chats
	ExternalChatID *string          `json:"external_chat_id,omitempty"` // Optional, auto-generated if not provided
	InitialMessage *string          `json:"initial_message,omitempty"`  // Optional first user message
	Configuration  *json.RawMessage `json:"configuration,omitempty"`    // Optional configuration JSON
	// InitialMedia   []MediaAttachment `json:"initial_media,omitempty"`    // Reverted: Optional media attachments for initial message
}

// ChatResponse defines the standard representation of a chat in API responses.
type ChatResponse struct {
	ID             uuid.UUID        `json:"id"`
	ChatbotID      uuid.UUID        `json:"chatbot_id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	InterfaceID    uuid.UUID        `json:"interface_id"`
	ExternalChatID string           `json:"external_chat_id"`
	Chat           []ChatMessage    `json:"chat"` // Changed from Messages to Chat, as required
	Feedback       *int8            `json:"feedback,omitempty"`
	Status         string           `json:"status"`
	Configuration  *json.RawMessage `json:"configuration,omitempty"` // Configuration data for this chat
	Chatbot        *ChatbotResponse `json:"chatbot,omitempty"`       // For detail views, include related chatbot
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// ListChatsResponse defines the response structure for listing chats.
type ListChatsResponse struct {
	Chats []ChatResponse `json:"chats"`
}

// AddMessageRequest defines the payload for adding a message to a chat.
type AddMessageRequest struct {
	Message string `json:"message"` // The user message to add
}

// UpdateChatFeedbackRequest defines the payload for updating chat feedback.
type UpdateChatFeedbackRequest struct {
	Feedback int8 `json:"feedback"` // -1 (negative), 0 (neutral), 1 (positive)
}

// AddMessageAsUserRequest defines the payload for adding a user message to a chat.
type AddMessageAsUserRequest struct {
	Message string `json:"message"` // The user message to add
	// Media   []MediaAttachment `json:"media,omitempty"` // Reverted: Optional media attachments
}

// AddMessageAsAssistantRequest defines the payload for adding an assistant message to a chat.
type AddMessageAsAssistantRequest struct {
	Message         string           `json:"message"`                     // The assistant message content
	Metadata        *json.RawMessage `json:"metadata,omitempty"`          // Optional metadata for the message
	SendToInterface *bool            `json:"send_to_interface,omitempty"` // Optional flag to send message to interface
	// Media    []MediaAttachment `json:"media,omitempty"`    // Reverted: Optional media attachments
}
