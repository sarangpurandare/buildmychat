package models

import (
	"time"
)

// Message represents a single message in a conversation.
// This structure is typically part of what's stored in the JSONB Messages field in the 'chats' table.
type Message struct {
	Role      string                 `json:"role"`               // e.g., "user", "assistant", "system"
	Content   string                 `json:"content"`            // The text content of the message
	Timestamp time.Time              `json:"timestamp"`          // Time the message was recorded
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // Optional metadata
}
