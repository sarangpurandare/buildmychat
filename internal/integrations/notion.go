package integrations

import (
	integration_models "buildmychat-backend/internal/models/integrations"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jomei/notionapi"
)

// Ensure NotionIntegration implements the Integration interface.
var _ Integration = (*NotionIntegration)(nil)

// NotionIntegration handles Notion-specific logic.
type NotionIntegration struct {
	// Add any Notion-specific dependencies here if needed later (e.g., Notion API client)
}

// NewNotionIntegration creates a new Notion integration handler.
func NewNotionIntegration() *NotionIntegration {
	return &NotionIntegration{}
}

// ValidateConfig checks if the provided JSON conforms to the NotionKBConfig structure
// and ensures at least one Notion Object ID is provided.
func (n *NotionIntegration) ValidateConfig(configJSON json.RawMessage) error {
	var config integration_models.NotionKBConfig

	if len(configJSON) == 0 || string(configJSON) == "null" {
		// Consider empty/null config valid, perhaps implying default behavior later?
		// For now, let's require object IDs.
		return errors.New("notion configuration cannot be empty, 'notion_object_ids' is required")
	}

	err := json.Unmarshal(configJSON, &config)
	if err != nil {
		return fmt.Errorf("invalid JSON format for Notion configuration: %w", err)
	}

	// Specific Notion validation
	if len(config.NotionObjectIDs) == 0 {
		return errors.New("'notion_object_ids' must contain at least one Notion Page or Database ID")
	}

	// TODO: Potentially add validation for the format of Notion Object IDs (UUID format)

	return nil // Configuration is valid
}

// TestConnection tests the connection to Notion using the API key.
func (n *NotionIntegration) TestConnection(ctx context.Context, decryptedCreds integration_models.DecryptedCredentials) (*integration_models.TestConnectionResult, error) {
	integrationSecret, ok := decryptedCreds["internal_integration_secret"] // Use correct key
	if !ok || integrationSecret == "" {
		// Return specific error type or message?
		return &integration_models.TestConnectionResult{
			Success: false,
			Message: "Missing or empty 'internal_integration_secret' in credentials", // Updated message
		}, nil // Not a connection error, but a credential format error
	}

	// 1. Instantiate a Notion client using the integrationSecret.
	client := notionapi.NewClient(notionapi.Token(integrationSecret))

	// 2. Make a simple read request to verify the key.
	// Getting the bot's own user info is a good, low-impact test.
	botUser, err := client.User.Me(ctx)

	// 3. Handle potential errors from the Notion API call.
	if err != nil {
		var notionErr *notionapi.Error
		if errors.As(err, &notionErr) {
			// Handle specific Notion API errors (e.g., unauthorized)
			message := fmt.Sprintf("Notion API error (%s): %s", notionErr.Code, notionErr.Message)
			// Check for common authentication error
			if notionErr.Status == 401 {
				message = "Notion API Error: Invalid API key (Unauthorized)."
			}
			return &integration_models.TestConnectionResult{
				Success: false,
				Message: message,
			}, nil // API error, not a system error
		}
		// Handle other potential errors (network, context deadline, etc.)
		return nil, fmt.Errorf("failed during Notion connection test: %w", err) // Return as system error
	}

	// If no error, the connection is successful
	// Extract bot name
	var botName string
	if botUser != nil && botUser.Type == notionapi.UserTypeBot && botUser.Bot != nil {
		// Note: The user object returned by Me() is the bot itself.
		// The Name field on the user object corresponds to the integration name.
		botName = botUser.Name
	}

	return &integration_models.TestConnectionResult{
		Success: true,
		Message: fmt.Sprintf("Successfully connected to Notion and verified token for Bot: '%s'", botName),
		Details: map[string]interface{}{"bot_name": botName},
	}, nil
}

// GetCredentialSchema returns an empty NotionCredentials struct to define the expected credential keys.
func (n *NotionIntegration) GetCredentialSchema() interface{} {
	return integration_models.NotionCredentials{}
}
