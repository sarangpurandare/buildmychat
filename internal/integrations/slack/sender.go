package slack

import (
	"buildmychat-backend/internal/store"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/slack-go/slack" // Import the slack package
	// "github.com/slack-go/slack" // Example import for actual implementation
)

// SlackConfig represents the configuration structure for Slack interfaces
type SlackConfig struct {
	BotToken      string `json:"bot_token"`
	SigningSecret string `json:"signing_secret"`
}

// ExtractTokenFromConfig extracts the bot token from a JSON configuration
func ExtractTokenFromConfig(configJSON json.RawMessage) (string, error) {
	if configJSON == nil {
		return "", fmt.Errorf("configuration is empty")
	}

	var config SlackConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return "", fmt.Errorf("failed to unmarshal Slack configuration: %w", err)
	}

	if config.BotToken == "" {
		return "", fmt.Errorf("bot_token not found in configuration")
	}

	return config.BotToken, nil
}

// SendMessageToChannel sends a message to a specified Slack channel using a bot token.
// If threadTs is provided, the message will be sent as a reply in a thread.
func SendMessageToChannel(ctx context.Context, botToken string, channelID string, text string, threadTs string) error {
	fmt.Printf("INFO - integrations.slack.SendMessageToChannel: Would send to Slack channel %s (using token starting with %.4s...): Message: '%s'",
		channelID, botToken, text)

	if threadTs != "" {
		fmt.Printf(" as a reply to thread %s\n", threadTs)
	} else {
		fmt.Println() // Just add a newline
	}

	// Implement actual Slack message sending using the Slack API client.
	if botToken == "" || botToken == "xoxb-dummy-placeholder-token" {
		return fmt.Errorf("SendMessageToChannel: invalid or placeholder bot token provided")
	}

	// Create a new Slack client
	apiClient := slack.New(botToken)

	// Prepare message options
	msgOptions := []slack.MsgOption{
		slack.MsgOptionText(text, false),
	}

	// If thread_ts is provided, add it to the message options
	if threadTs != "" {
		msgOptions = append(msgOptions, slack.MsgOptionTS(threadTs))
	}

	// Send the message to Slack
	_, _, err := apiClient.PostMessageContext(ctx, channelID, msgOptions...)
	if err != nil {
		return fmt.Errorf("failed to post message to Slack channel %s: %w", channelID, err)
	}

	return nil
}

// SendMessageUsingInterfaceConfig sends a message to Slack using the interface's configuration
func SendMessageUsingInterfaceConfig(ctx context.Context, configJSON json.RawMessage, channelID string, text string, threadTs string) error {
	botToken, err := ExtractTokenFromConfig(configJSON)
	if err != nil {
		return fmt.Errorf("failed to extract bot token: %w", err)
	}

	return SendMessageToChannel(ctx, botToken, channelID, text, threadTs)
}

// SendMessageUsingInterfaceID sends a message to Slack using the interface ID to fetch configuration
func SendMessageUsingInterfaceID(ctx context.Context, store store.Store, interfaceID uuid.UUID, orgID uuid.UUID, channelID string, text string, threadTs string) error {
	// Get interface by ID
	intf, err := store.GetInterfaceByID(ctx, interfaceID, orgID)
	if err != nil {
		return fmt.Errorf("failed to fetch interface with ID %s: %w", interfaceID, err)
	}

	// Extract token from interface configuration
	botToken, err := ExtractTokenFromConfig(intf.Configuration)
	if err != nil {
		return fmt.Errorf("failed to extract bot token from interface %s: %w", interfaceID, err)
	}

	// Send the message
	return SendMessageToChannel(ctx, botToken, channelID, text, threadTs)
}
