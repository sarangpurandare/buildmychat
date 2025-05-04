package integrations

// Defines the expected configuration structure for a Notion Knowledge Base.
type NotionKBConfig struct {
	NotionObjectIDs []string `json:"notion_object_ids"` // List of Notion Page or Database IDs to index.
	// Add other Notion-specific config fields here if needed, e.g., SyncStatus
	SyncStatus string `json:"sync_status,omitempty"` // e.g., PENDING, SYNCING, COMPLETED, FAILED
}

// Defines the expected configuration structure for a Slack Interface.
type SlackInterfaceConfig struct {
	SlackTeamID string `json:"slack_team_id"` // The Slack Workspace/Team ID.
	// Add other Slack-specific config fields here, e.g., default channel, app ID?
}

// Defines the expected structure for Notion API credentials (stored encrypted).
type NotionCredentials struct {
	InternalIntegrationSecret string `json:"internal_integration_secret"` // Correct key name for Notion token
}

// Defines the expected structure for Slack API credentials (stored encrypted).
type SlackCredentials struct {
	BotToken      string `json:"bot_token"`               // xoxb-... token
	SigningSecret string `json:"signing_secret"`          // Used for webhook verification
	ClientID      string `json:"client_id,omitempty"`     // Optional: For OAuth flow if implemented later
	ClientSecret  string `json:"client_secret,omitempty"` // Optional: For OAuth flow if implemented later
}

// Represents the standard structure for testing an integration's connection.
type TestConnectionResult struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"` // Optional message, e.g., error details or success confirmation
	Details map[string]interface{} `json:"details,omitempty"` // Optional map for extra details (e.g., {"bot_name": "..."})
}

// Helper type for decrypted credentials map
type DecryptedCredentials map[string]string
