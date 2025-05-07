package slack

import (
	"context"
	"encoding/json"
	"fmt"
)

// This is an example file showing how to use the Slack integration functions.
// These are not actual tests but examples of usage.

func ExampleSendMessageUsingInterfaceConfig() {
	// Example interface configuration JSON
	configJSON := []byte(`{
		"bot_token": "xoxb-your-token-here",
		"signing_secret": "your-signing-secret-here"
	}`)

	// Channel ID to send the message to
	channelID := "C12345678"

	// Message to send
	message := "Hello from BuildMyChat!"

	// Optional thread timestamp (for replying in a thread)
	threadTs := ""

	// Send the message
	err := SendMessageUsingInterfaceConfig(context.Background(), configJSON, channelID, message, threadTs)
	if err != nil {
		fmt.Printf("Error sending message: %v\n", err)
		return
	}

	fmt.Println("Message sent successfully!")
}

func ExampleExtractTokenFromConfig() {
	// Example interface configuration JSON
	configJSON := []byte(`{
		"bot_token": "xoxb-example-token",
		"signing_secret": "example-secret"
	}`)

	// Extract the bot token
	token, err := ExtractTokenFromConfig(configJSON)
	if err != nil {
		fmt.Printf("Error extracting token: %v\n", err)
		return
	}

	fmt.Printf("Extracted token: %s\n", token)
	// Output: Extracted token: xoxb-example-token
}

func ExampleSlackConfig() {
	// Create a SlackConfig struct
	config := SlackConfig{
		BotToken:      "xoxb-example-token",
		SigningSecret: "example-secret",
	}

	// Marshal to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		fmt.Printf("Error marshaling config: %v\n", err)
		return
	}

	fmt.Printf("Config JSON: %s\n", string(configJSON))
	// Output: Config JSON: {"bot_token":"xoxb-example-token","signing_secret":"example-secret"}
}
