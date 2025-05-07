package models

// SlackEventPayload represents the overall structure of an event callback from Slack.
type SlackEventPayload struct {
	Token              string          `json:"token"`
	TeamID             string          `json:"team_id"`
	APIAppID           string          `json:"api_app_id"`
	Event              SlackEvent      `json:"event"`
	Type               string          `json:"type"` // e.g., "event_callback"
	EventID            string          `json:"event_id"`
	EventTime          int64           `json:"event_time"`
	Authorizations     []Authorization `json:"authorizations"`
	IsExtSharedChannel bool            `json:"is_ext_shared_channel"`
	EventContext       string          `json:"event_context"`
}

// SlackEvent represents the actual event details within the payload.
type SlackEvent struct {
	User        string  `json:"user"` // User ID of the sender
	Type        string  `json:"type"` // e.g., "message", "app_mention"
	Text        string  `json:"text"` // Message content
	Timestamp   string  `json:"ts"`   // Timestamp of the message
	ClientMsgID string  `json:"client_msg_id"`
	Team        string  `json:"team"`    // Team ID where the event occurred
	Blocks      []Block `json:"blocks"`  // Rich text blocks
	Channel     string  `json:"channel"` // Channel ID where the message was sent
	EventTs     string  `json:"event_ts"`
	ChannelType string  `json:"channel_type"`
}

// Authorization represents an authorization entry in the Slack event payload.
type Authorization struct {
	EnterpriseID        *string `json:"enterprise_id"`
	TeamID              string  `json:"team_id"`
	UserID              string  `json:"user_id"` // User ID of the bot/app
	IsBot               bool    `json:"is_bot"`
	IsEnterpriseInstall bool    `json:"is_enterprise_install"`
}

// Block represents a block in the Slack message's rich text formatting.
type Block struct {
	Type     string    `json:"type"`
	BlockID  string    `json:"block_id"`
	Elements []Element `json:"elements"`
}

// Element represents an element within a Slack message block.
type Element struct {
	Type     string       `json:"type"`
	Elements []SubElement `json:"elements,omitempty"` // Used in rich_text_section
	UserID   string       `json:"user_id,omitempty"`  // Used in "user" type
	Text     string       `json:"text,omitempty"`     // Used in "text" type
}

// SubElement is for nested elements within a rich_text_section.
type SubElement struct {
	Type   string `json:"type"`
	UserID string `json:"user_id,omitempty"`
	Text   string `json:"text,omitempty"`
}

// SlackChallengeRequest is used for Slack's URL verification.
type SlackChallengeRequest struct {
	Token     string `json:"token"`
	Challenge string `json:"challenge"`
	Type      string `json:"type"` // "url_verification"
}
