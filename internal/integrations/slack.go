package integrations

import (
	integration_models "buildmychat-backend/internal/models/integrations"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/slack-go/slack"
)

// Ensure SlackIntegration implements the Integration interface.
var _ Integration = (*SlackIntegration)(nil)

// SlackIntegration handles Slack-specific logic.
type SlackIntegration struct {
	// Add any Slack-specific dependencies here if needed later (e.g., Slack API client)
}

// NewSlackIntegration creates a new Slack integration handler.
func NewSlackIntegration() *SlackIntegration {
	return &SlackIntegration{}
}

// ValidateConfig checks if the provided JSON conforms to the SlackInterfaceConfig structure.
func (s *SlackIntegration) ValidateConfig(configJSON json.RawMessage) error {
	var config integration_models.SlackInterfaceConfig

	if len(configJSON) == 0 || string(configJSON) == "null" {
		// Assuming empty/null config is valid for Slack interface (might not need specific config initially)
		return nil
	}

	err := json.Unmarshal(configJSON, &config)
	if err != nil {
		return fmt.Errorf("invalid JSON format for Slack configuration: %w", err)
	}

	// Specific Slack validation
	if config.SlackTeamID == "" {
		// Making SlackTeamID optional for now, adjust if required
		// return errors.New("'slack_team_id' is required in Slack configuration")
	}

	return nil // Configuration is valid
}

// TestConnection tests the connection to Slack using the bot token.
func (s *SlackIntegration) TestConnection(ctx context.Context, decryptedCreds integration_models.DecryptedCredentials) (*integration_models.TestConnectionResult, error) {
	botToken, tokenOk := decryptedCreds["bot_token"] // Matches SlackCredentials json tag
	// Signing secret is not strictly needed for auth.test, but good to check presence
	_, secretOk := decryptedCreds["signing_secret"]

	if !tokenOk || botToken == "" {
		return &integration_models.TestConnectionResult{
			Success: false,
			Message: "Missing or empty 'bot_token' in Slack credentials",
		}, nil
	}
	if !secretOk {
		// Warn but proceed? Or fail? Let's warn for now, as auth.test only needs the token.
		fmt.Println("WARN [SlackIntegration] TestConnection: Missing 'signing_secret' in provided credentials. Webhooks will fail.")
	}

	// 1. Instantiate a Slack client using the botToken.
	client := slack.New(botToken)

	// 2. Make a simple API call like `auth.test` to verify the token.
	authTestResponse, err := client.AuthTestContext(ctx)
	if err != nil {
		// Check for specific Slack API errors by inspecting the error message
		errStr := err.Error()
		if strings.Contains(errStr, "invalid_auth") { // Check error string
			return &integration_models.TestConnectionResult{
				Success: false,
				Message: "Slack API Error: Invalid authentication token (bot_token).",
			}, nil
		} else if strings.Contains(errStr, "not_authed") { // Check error string
			return &integration_models.TestConnectionResult{
				Success: false,
				Message: "Slack API Error: Not authenticated (check token scopes?).",
			}, nil
		} // Add more checks for other common errors like 'account_inactive' if needed

		// Handle other potential errors (network, context deadline, etc.)
		log.Printf("ERROR [SlackIntegration] TestConnection: Unhandled Slack API error or system error: %v", err)
		return nil, fmt.Errorf("failed during Slack connection test (AuthTest): %w", err)
	}

	// 3. If successful, extract useful details
	botUserID := authTestResponse.UserID
	teamID := authTestResponse.TeamID
	botName := authTestResponse.User // This is usually the bot's display name

	return &integration_models.TestConnectionResult{
		Success: true,
		Message: fmt.Sprintf("Successfully connected to Slack workspace '%s' and verified token for Bot '%s' (ID: %s)", authTestResponse.Team, botName, botUserID),
		Details: map[string]interface{}{
			"bot_name":    botName,
			"bot_user_id": botUserID,
			"team_id":     teamID, // Could be useful for cross-checking config
		},
	}, nil
}

// GetCredentialSchema returns an empty SlackCredentials struct to define the expected credential keys.
func (s *SlackIntegration) GetCredentialSchema() interface{} {
	return integration_models.SlackCredentials{}
}
