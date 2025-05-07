package handlers

import (
	"buildmychat-backend/internal/models"
	"buildmychat-backend/internal/services"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// SlackWebhookHandlers handles incoming Slack webhook events.
type SlackWebhookHandlers struct {
	chatService *services.ChatService
	// We might need ChatbotService later for fetching chatbot-specific Slack signing secrets
	// chatbotService *services.ChatbotService
}

// NewSlackWebhookHandlers creates a new SlackWebhookHandlers instance.
func NewSlackWebhookHandlers(cs *services.ChatService) *SlackWebhookHandlers {
	return &SlackWebhookHandlers{
		chatService: cs,
	}
}

// HandleSlackEvent handles incoming events from Slack.
// The URL for this handler will be like /v1/slack-events/{chatbot_id}
func (h *SlackWebhookHandlers) HandleSlackEvent(w http.ResponseWriter, r *http.Request) {
	// 1. Extract chatbot_id from URL
	chatbotIDStr := chi.URLParam(r, "chatbotID") // Ensure router uses "chatbotID"
	chatbotID, err := uuid.Parse(chatbotIDStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid chatbot ID in URL")
		return
	}
	fmt.Printf("DEBUG - HandleSlackEvent - ChatbotID from URL: %s\n", chatbotID)

	// 2. Read the request body once
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	// Restore the body for subsequent reads if necessary (though we'll unmarshal based on type)
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 3. Determine payload type (url_verification or event_callback)
	var typeFinder struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(bodyBytes, &typeFinder); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Could not determine payload type: "+err.Error())
		return
	}

	fmt.Printf("DEBUG - HandleSlackEvent - Detected payload type: %s\n", typeFinder.Type)

	// 4. Handle based on type
	if typeFinder.Type == "url_verification" {
		var challengeReq models.SlackChallengeRequest
		if err := json.Unmarshal(bodyBytes, &challengeReq); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid Slack challenge request: "+err.Error())
			return
		}
		fmt.Printf("DEBUG - HandleSlackEvent - Responding to Slack URL verification challenge: %s\n", challengeReq.Challenge)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(challengeReq.Challenge))
		return
	}

	if typeFinder.Type == "event_callback" {
		var payload models.SlackEventPayload
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid Slack event payload: "+err.Error())
			return
		}

		// We are interested in "message" or "app_mention" event types within an "event_callback".
		if payload.Event.Type != "message" && payload.Event.Type != "app_mention" {
			fmt.Printf("DEBUG - HandleSlackEvent - Ignoring event type: %s for event_callback\n", payload.Event.Type)
			w.WriteHeader(http.StatusOK) // Acknowledge other event types we don't handle yet
			json.NewEncoder(w).Encode(map[string]string{"status": "event type ignored"})
			return
		}

		// 5. Extract relevant details from event_callback
		teamID := payload.TeamID
		channelID := payload.Event.Channel
		userID := payload.Event.User // User who sent the message
		text := payload.Event.Text
		eventType := payload.Event.Type

		if teamID == "" || channelID == "" || userID == "" {
			errMsg := fmt.Sprintf("Missing crucial IDs from event_callback: team_id: '%s', channel_id: '%s', user_id: '%s'", teamID, channelID, userID)
			fmt.Printf("DEBUG - HandleSlackEvent - Error: %s\n", errMsg)
			RespondWithError(w, http.StatusBadRequest, "Missing team_id, channel_id, or user_id in event_callback payload")
			return
		}

		externalChatID := fmt.Sprintf("%s_%s_%s", teamID, channelID, userID)

		fmt.Printf("DEBUG - HandleSlackEvent - Received Slack Event:\n")
		fmt.Printf("  ChatbotID (from URL): %s\n", chatbotID)
		fmt.Printf("  EventType: %s\n", eventType)
		fmt.Printf("  TeamID: %s\n", teamID)
		fmt.Printf("  ChannelID: %s\n", channelID)
		fmt.Printf("  UserID (event sender): %s\n", userID)
		fmt.Printf("  Text: %s\n", text)
		fmt.Printf("  ExternalChatID (constructed): %s\n", externalChatID)

		// TODO: Implement Slack request signature verification here. CRITICAL for security.
		// This will involve:
		// 1. Getting the Slack Signing Secret for this chatbot/interface_node.
		// 2. Verifying the signature on `bodyBytes` using headers from `r.Header`.

		// --- STAGE 2: Find or Create Chat, Process AI, Send Reply ---

		// 6. Find or Create Chat
		// Placeholder: orgID needs to be determined, perhaps from chatbotID or a default for the webhook
		// For now, let's assume we can get orgID if needed by ChatService, or it's implicit.
		// A user's authentication context isn't directly available in a webhook.
		// The chatbot itself belongs to an organization.
		orgIDForChatbot, err := h.chatService.GetOrgIDForChatbot(r.Context(), chatbotID)
		if err != nil {
			fmt.Printf("DEBUG - HandleSlackEvent - Error getting orgID for chatbot %s: %v\n", chatbotID, err)
			// Decide if this is a critical error. For now, we might not need orgID directly if
			// ChatService methods can operate with just chatbotID for external interactions.
			// However, most services are org-scoped.
			// This implies chatService needs a way to get/validate chatbot without orgID from context.
			// Or, the chatbot's orgID is retrieved and used.
			RespondWithError(w, http.StatusInternalServerError, "Could not determine organization for chatbot.")
			return
		}

		initialUserMessage := models.Message{
			Role:      "user",
			Content:   text,
			Timestamp: time.Now().UTC(),
		}

		// Create a configuration JSON with thread_ts for Slack threading
		var configJSON json.RawMessage
		if threadTs := payload.Event.Timestamp; threadTs != "" {
			config, err := json.Marshal(map[string]string{
				"thread_ts": threadTs,
			})
			if err == nil {
				configJSON = config
				fmt.Printf("DEBUG - HandleSlackEvent - Thread TS '%s' stored in config\n", threadTs)
			} else {
				fmt.Printf("DEBUG - HandleSlackEvent - Error marshaling config JSON: %v\n", err)
			}
		}

		chat, err := h.chatService.FindOrCreateChatForExternalID(r.Context(), orgIDForChatbot, chatbotID, externalChatID, initialUserMessage, configJSON)
		if err != nil {
			fmt.Printf("DEBUG - HandleSlackEvent - Error finding/creating chat: %v\n", err)
			RespondWithError(w, http.StatusInternalServerError, "Failed to process chat session: "+err.Error())
			return
		}
		fmt.Printf("DEBUG - HandleSlackEvent - Found/Created chat ID: %s\n", chat.ID)

		// 7. Process AI Reply (Dummy)
		// The initial message is already added by FindOrCreateChatForExternalID if the chat was new
		// or we can add it explicitly if FindOrCreateChatForExternalID only finds/creates the container.
		// Let's assume FindOrCreateChatForExternalID adds the first message if creating.
		// If it only finds, we might need to add the current message:
		// if !chat.IsNew { // Hypothetical field
		//    _, err = h.chatService.AddMessageToChat(r.Context(), orgIDForChatbot, chat.ID, models.Message{Role: "user", Content: text})
		//    if err != nil { ... }
		// }

		aiResponseContent := h.processAIDummyReply(chatbotID, chat.ID, text)
		fmt.Printf("DEBUG - HandleSlackEvent - Dummy AI Response: %s\n", aiResponseContent)

		// 8. Add AI's message to our chat history and send to interface
		// Create the request with SendToInterface set to true
		assistantReq := models.AddMessageAsAssistantRequest{
			Message:         aiResponseContent,
			SendToInterface: func() *bool { b := true; return &b }(), // Set to true
		}

		// Create a JSON request body
		reqBody, err := json.Marshal(assistantReq)
		if err != nil {
			fmt.Printf("DEBUG - HandleSlackEvent - Error marshaling assistant request: %v\n", err)
			RespondWithError(w, http.StatusInternalServerError, "Failed to process AI response")
			return
		}

		// Create a request to the chat handler
		req, err := http.NewRequestWithContext(r.Context(), "POST",
			fmt.Sprintf("/v1/chats/%s/messages/assistant", chat.ID), bytes.NewBuffer(reqBody))
		if err != nil {
			fmt.Printf("DEBUG - HandleSlackEvent - Error creating request: %v\n", err)
			RespondWithError(w, http.StatusInternalServerError, "Failed to process AI response")
			return
		}

		// Set the organization ID in the request context
		req = req.WithContext(context.WithValue(req.Context(), "organization_id", orgIDForChatbot))

		// Call the chat handler directly
		chatHandler := NewChatHandlers(h.chatService)
		recorder := httptest.NewRecorder()
		chatHandler.HandleAddAssistantMessage(recorder, req)

		// Check the response
		if recorder.Code != http.StatusOK {
			fmt.Printf("DEBUG - HandleSlackEvent - Error from chat handler: %d - %s\n",
				recorder.Code, recorder.Body.String())
			// Continue anyway to acknowledge the webhook
		}

		// Acknowledge the event to Slack.
		// This should be done quickly, ideally before long-running AI processing.
		// The current structure processes then ACKs. For long AI tasks, an async model is better.
		// For this iterative step, direct ACK after processing is fine.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "event processed"})
		return
	}

	// Fallback for unhandled types
	fmt.Printf("DEBUG - HandleSlackEvent - Unhandled payload type: %s\n", typeFinder.Type)
	RespondWithError(w, http.StatusBadRequest, "Unhandled payload type: "+typeFinder.Type)
}

// processAIDummyReply is a placeholder for actual AI processing.
func (h *SlackWebhookHandlers) processAIDummyReply(chatbotID uuid.UUID, chatID uuid.UUID, userMessage string) string {
	fmt.Printf("INFO - processAIDummyReply - Args: chatbotID=%s, chatID=%s, userMessage='%s'\n", chatbotID, chatID, userMessage)
	return fmt.Sprintf("Acknowledged your message: '%s'. (Processed by chatbot %s for chat %s)", userMessage, chatbotID, chatID)
}
