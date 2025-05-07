package services

import (
	"buildmychat-backend/internal/integrations/slack"
	"buildmychat-backend/internal/models"
	"buildmychat-backend/internal/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"bytes"

	"github.com/google/uuid"
)

// ChatService handles chat-related business logic.
type ChatService struct {
	store             store.Store
	chatbotService    *ChatbotService
	credentialService CredentialsService
}

// NewChatService creates a new ChatService.
func NewChatService(store store.Store, chatbotService *ChatbotService, credentialService CredentialsService) *ChatService {
	return &ChatService{
		store:             store,
		chatbotService:    chatbotService,
		credentialService: credentialService,
	}
}

// mapChatToResponse converts a DB chat model to an API response DTO.
func (s *ChatService) mapChatToResponse(ctx context.Context, dbChat *models.Chat, includeChatbot bool) (*models.ChatResponse, error) {
	// Parse chat data as messages
	var messages []models.ChatMessage
	if err := json.Unmarshal(dbChat.ChatData, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse chat data: %w", err)
	}

	// Convert configuration to pointer for JSON encoding
	var configPtr *json.RawMessage
	if len(dbChat.Configuration) > 0 && !bytes.Equal(dbChat.Configuration, []byte("{}")) && !bytes.Equal(dbChat.Configuration, []byte("null")) {
		configPtr = &dbChat.Configuration
	}

	resp := &models.ChatResponse{
		ID:             dbChat.ID,
		ChatbotID:      dbChat.ChatbotID,
		OrganizationID: dbChat.OrganizationID,
		InterfaceID:    dbChat.InterfaceID,
		ExternalChatID: dbChat.ExternalChatID,
		Chat:           messages,
		Feedback:       dbChat.Feedback,
		Status:         dbChat.Status,
		Configuration:  configPtr,
		CreatedAt:      dbChat.CreatedAt,
		UpdatedAt:      dbChat.UpdatedAt,
	}

	// Optionally include chatbot details
	if includeChatbot && s.chatbotService != nil {
		chatbot, err := s.chatbotService.GetChatbotByID(ctx, dbChat.OrganizationID, dbChat.ChatbotID)
		if err == nil {
			resp.Chatbot = chatbot
		}
		// If there's an error getting the chatbot, we'll just leave it as nil
	}

	return resp, nil
}

// CreateChat creates a new chat associated with a chatbot.
func (s *ChatService) CreateChat(ctx context.Context, orgID uuid.UUID, req models.CreateChatRequest) (*models.ChatResponse, error) {
	// Debug logging
	fmt.Printf("DEBUG - ChatService.CreateChat - orgID: %s, chatbotID: %s\n", orgID, req.ChatbotID)

	// 1. Validate required ChatbotID
	if req.ChatbotID == uuid.Nil {
		return nil, fmt.Errorf("chatbot_id is required")
	}

	// 2. Ensure the chatbot itself exists for the given organization.
	// This call also implicitly validates orgID linkage to chatbotID.
	chatbot, chatbotErr := s.chatbotService.GetChatbotByID(ctx, orgID, req.ChatbotID)
	if chatbotErr != nil {
		fmt.Printf("DEBUG - ChatService.CreateChat - Error getting chatbot: %v\n", chatbotErr)
		if errors.Is(chatbotErr, store.ErrNotFound) {
			return nil, fmt.Errorf("chatbot with ID %s not found for organization %s", req.ChatbotID, orgID)
		}
		return nil, fmt.Errorf("failed to verify chatbot (ID: %s) existence: %w", req.ChatbotID, chatbotErr)
	}
	fmt.Printf("DEBUG - ChatService.CreateChat - Found chatbot: %s (OrgID: %s)\n", chatbot.Name, chatbot.OrganizationID)

	var determinedInterfaceID uuid.UUID // Defaults to uuid.Nil. This will be stored if no specific interface is linked.

	if req.InterfaceID != nil { // An interface_id key was present in the request.
		potentialInterfaceID := *req.InterfaceID

		if potentialInterfaceID != uuid.Nil { // A specific, non-nil interface ID was provided. Validate it.
			mappings, err := s.store.GetChatbotMappings(ctx, req.ChatbotID, orgID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) { // Chatbot exists but has no mappings.
					return nil, fmt.Errorf("chatbot (ID: %s) has no associated interfaces, so provided interface_id %s cannot be validated as linked", req.ChatbotID, potentialInterfaceID)
				}
				return nil, fmt.Errorf("failed to retrieve mappings for chatbot (ID: %s) to validate provided interface_id %s: %w", req.ChatbotID, potentialInterfaceID, err)
			}

			found := false
			// Check if mappings is nil before trying to iterate (though GetChatbotMappings should ideally not return (nil, nil))
			if mappings != nil && mappings.Interfaces != nil {
				for _, ifaceNode := range mappings.Interfaces {
					if ifaceNode.ID == potentialInterfaceID {
						// Additionally, ensure the interface node itself is active if your model supports IsActive for nodes.
						// For now, just matching ID is sufficient based on current structure.
						found = true
						break
					}
				}
			}

			if !found {
				return nil, fmt.Errorf("provided interface_id %s is not valid or not actively associated with chatbot %s", potentialInterfaceID, req.ChatbotID)
			}
			determinedInterfaceID = potentialInterfaceID // Validation passed, use this specific ID.
		}
		// If potentialInterfaceID was uuid.Nil (client sent "0000..."),
		// determinedInterfaceID remains its default uuid.Nil, signifying no specific interface.
	}
	// If req.InterfaceID was nil (key not in JSON), determinedInterfaceID also remains its default uuid.Nil.

	// Create initial messages
	messages := []models.ChatMessage{
		{
			Role:      "system",
			Content:   "This is the beginning of your conversation",
			Timestamp: time.Now().Unix(),
			SentBy:    "system",
			Hide:      1, // System messages are hidden by default
		},
	}

	// Add the user's initial message if provided
	if req.InitialMessage != nil {
		userMessage := models.ChatMessage{
			Role:      "user",
			Content:   *req.InitialMessage,
			Timestamp: time.Now().Unix(),
			SentBy:    "user",
			Hide:      0,
		}
		messages = append(messages, userMessage)
	}

	// Marshal messages to JSON for storage
	messagesJSON, err := json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal initial messages: %w", err)
	}

	externalChatID := ""
	if req.ExternalChatID != nil {
		externalChatID = *req.ExternalChatID
	}

	// Prepare configuration JSON
	var configJSON []byte
	if req.Configuration != nil {
		configJSON = *req.Configuration
	} else {
		configJSON = []byte("{}")
	}

	// Create the chat in the database
	dbChat := &models.Chat{
		ID:             uuid.New(), // Generate a new UUID
		ChatbotID:      req.ChatbotID,
		OrganizationID: orgID,
		InterfaceID:    determinedInterfaceID,
		ExternalChatID: externalChatID,
		ChatData:       messagesJSON,
		Status:         "ACTIVE",
		Configuration:  configJSON,
	}

	// Create the chat in the database
	params := store.CreateChatParams{
		ID:             dbChat.ID,
		OrganizationID: orgID,
		ChatbotID:      req.ChatbotID,
		InterfaceID:    determinedInterfaceID,
		ExternalChatID: externalChatID,
		ChatData:       messagesJSON,
		Configuration:  configJSON,
	}

	createdChat, err := s.store.CreateChat(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat in store: %w", err)
	}

	// Convert to API response
	resp, err := s.mapChatToResponse(ctx, createdChat, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat response: %w", err)
	}

	return resp, nil
}

// GetChatByID retrieves a specific chat by its ID.
func (s *ChatService) GetChatByID(ctx context.Context, orgID, chatID uuid.UUID, includeChatbot bool) (*models.ChatResponse, error) {
	dbChat, err := s.store.GetChatByID(ctx, chatID, orgID)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, err // Propagate not found error
		}
		return nil, fmt.Errorf("failed to get chat from store: %w", err)
	}

	resp, err := s.mapChatToResponse(ctx, dbChat, includeChatbot)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat response: %w", err)
	}

	return resp, nil
}

// GetChatByExternalID retrieves a chat by its external ID and interface ID.
func (s *ChatService) GetChatByExternalID(ctx context.Context, orgID uuid.UUID, externalID string, interfaceID uuid.UUID, includeChatbot bool) (*models.ChatResponse, error) {
	dbChat, err := s.store.GetChatByExternalID(ctx, externalID, interfaceID, orgID)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, err // Propagate not found error
		}
		return nil, fmt.Errorf("failed to get chat from store: %w", err)
	}

	resp, err := s.mapChatToResponse(ctx, dbChat, includeChatbot)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat response: %w", err)
	}

	return resp, nil
}

// ListChatsByOrg retrieves all chats for an organization.
func (s *ChatService) ListChatsByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int, includeChatbot bool) (*models.ListChatsResponse, error) {
	// Set reasonable defaults for limit and offset
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	dbChats, err := s.store.ListChatsByOrg(ctx, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list chats from store: %w", err)
	}

	// Map each database chat to a response DTO
	responseChats := make([]models.ChatResponse, 0, len(dbChats))
	for i := range dbChats {
		chatResp, err := s.mapChatToResponse(ctx, &dbChats[i], includeChatbot)
		if err != nil {
			return nil, fmt.Errorf("failed to create chat response at index %d: %w", i, err)
		}
		responseChats = append(responseChats, *chatResp)
	}

	return &models.ListChatsResponse{Chats: responseChats}, nil
}

// ListChatsByChatbot retrieves all chats for a specific chatbot.
func (s *ChatService) ListChatsByChatbot(ctx context.Context, orgID, chatbotID uuid.UUID, limit, offset int, includeChatbot bool) (*models.ListChatsResponse, error) {
	// Set reasonable defaults for limit and offset
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	dbChats, err := s.store.ListChatsByChatbot(ctx, chatbotID, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list chats from store: %w", err)
	}

	// Map each database chat to a response DTO
	responseChats := make([]models.ChatResponse, 0, len(dbChats))
	for i := range dbChats {
		chatResp, err := s.mapChatToResponse(ctx, &dbChats[i], includeChatbot)
		if err != nil {
			return nil, fmt.Errorf("failed to create chat response at index %d: %w", i, err)
		}
		responseChats = append(responseChats, *chatResp)
	}

	return &models.ListChatsResponse{Chats: responseChats}, nil
}

// AddMessageToChat adds a user message to a chat and processes it using the chatbot.
func (s *ChatService) AddMessageToChat(ctx context.Context, orgID, chatID uuid.UUID, message string) (*models.ChatResponse, error) {
	// First get the chat to ensure it exists and belongs to the organization
	_, err := s.store.GetChatByID(ctx, chatID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	// Create a new user message
	userMessage := models.ChatMessage{
		Role:      "user",
		Content:   message,
		Timestamp: time.Now().Unix(), // Unix timestamp (seconds since epoch)
		SentBy:    "user",            // Set SentBy field to match Role
		Hide:      0,                 // Default to show
	}

	// Add the message to the chat
	if err := s.store.AddMessageToChat(ctx, chatID, userMessage, orgID); err != nil {
		return nil, fmt.Errorf("failed to add user message to chat: %w", err)
	}

	// TODO: In a real implementation, this would trigger an async job to process the message
	// and generate a response from the AI. For now, we'll just update the status and return.

	// Update the chat status to PROCESSING
	if err := s.store.UpdateChatStatus(ctx, chatID, "PROCESSING", orgID); err != nil {
		return nil, fmt.Errorf("failed to update chat status: %w", err)
	}

	// Get the updated chat to return
	updatedChat, err := s.store.GetChatByID(ctx, chatID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated chat: %w", err)
	}

	// Create the response
	resp, err := s.mapChatToResponse(ctx, updatedChat, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat response: %w", err)
	}

	return resp, nil
}

// AddAssistantMessageToChat adds an assistant message to a chat and updates its status to ACTIVE.
// If the chat has an associated interface, it will also send the message to that interface.
func (s *ChatService) AddAssistantMessageToChat(ctx context.Context, orgID, chatID uuid.UUID, message string, metadata *json.RawMessage) (*models.ChatResponse, error) {
	// First get the chat to ensure it exists and belongs to the organization
	chat, err := s.store.GetChatByID(ctx, chatID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	// Create a new assistant message
	assistantMessage := models.ChatMessage{
		Role:      "assistant",
		Content:   message,
		Timestamp: time.Now().Unix(), // Unix timestamp (seconds since epoch)
		SentBy:    "assistant",       // Set SentBy field to match Role
		Hide:      0,                 // Default to show
		Metadata:  metadata,
	}

	// Add the message to the chat
	if err := s.store.AddMessageToChat(ctx, chatID, assistantMessage, orgID); err != nil {
		return nil, fmt.Errorf("failed to add assistant message to chat: %w", err)
	}

	// Update the chat status to ACTIVE (ready for next user input)
	if err := s.store.UpdateChatStatus(ctx, chatID, "ACTIVE", orgID); err != nil {
		return nil, fmt.Errorf("failed to update chat status: %w", err)
	}

	// Check if this chat has an associated interface - if so, send the message to that interface
	if chat.InterfaceID != uuid.Nil {
		if err := s.sendMessageToInterface(ctx, chat, message); err != nil {
			// Log the error but don't fail the entire operation
			fmt.Printf("WARNING - AddAssistantMessageToChat - Failed to send message to interface: %v\n", err)
		}
	}

	// Get the updated chat to return
	updatedChat, err := s.store.GetChatByID(ctx, chatID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated chat: %w", err)
	}

	// Create the response
	resp, err := s.mapChatToResponse(ctx, updatedChat, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat response: %w", err)
	}

	return resp, nil
}

// sendMessageToInterface sends a message to the appropriate interface based on the interface type.
// Currently supports Slack interfaces.
func (s *ChatService) sendMessageToInterface(ctx context.Context, chat *models.Chat, message string) error {
	// Get the interface details to determine its type
	iface, err := s.store.GetInterfaceByID(ctx, chat.InterfaceID, chat.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get interface details: %w", err)
	}

	// Check the interface type and dispatch to the appropriate handler
	switch iface.ServiceType {
	case models.ServiceTypeSlack:
		return s.sendMessageToSlack(ctx, chat, iface, message)
	default:
		return fmt.Errorf("unsupported interface type: %s", iface.ServiceType)
	}
}

// sendMessageToSlack sends a message to a Slack channel.
func (s *ChatService) sendMessageToSlack(ctx context.Context, chat *models.Chat, iface *models.Interface, message string) error {
	// 1. Get the Slack credentials
	if iface.CredentialID == uuid.Nil {
		return fmt.Errorf("slack interface has no associated credential")
	}

	// Get the decrypted credentials using CredentialService
	credential, err := s.credentialService.GetDecryptedCredential(ctx, iface.CredentialID, chat.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to get Slack credentials: %w", err)
	}

	// Extract the bot token from the decrypted credentials
	var creds map[string]string
	if err := json.Unmarshal(credential.DecryptedCredentials, &creds); err != nil {
		return fmt.Errorf("failed to parse Slack credentials: %w", err)
	}

	botToken, ok := creds["bot_token"]
	if !ok || botToken == "" {
		return fmt.Errorf("invalid or missing Slack bot token in credentials")
	}

	// Extract channel ID from external_chat_id
	if chat.ExternalChatID == "" {
		return fmt.Errorf("chat has no external chat ID for Slack")
	}

	parts := strings.Split(chat.ExternalChatID, "_")
	if len(parts) < 2 {
		return fmt.Errorf("invalid external chat ID format: %s", chat.ExternalChatID)
	}

	channelID := parts[1] // channel_id is the second part

	// Extract thread_ts from configuration if available
	var threadTs string
	if chat.Configuration != nil && len(chat.Configuration) > 0 {
		var config map[string]string
		if err := json.Unmarshal(chat.Configuration, &config); err == nil {
			threadTs = config["thread_ts"]
			if threadTs != "" {
				fmt.Printf("INFO - ChatService.sendMessageToSlack - Using thread_ts '%s' from configuration\n", threadTs)
			}
		}
	}

	// Use the Slack integration to actually send the message
	fmt.Printf("INFO - ChatService.sendMessageToSlack - Sending message to Slack channel %s\n", channelID)

	// Import the slack integration package in the imports section
	return slack.SendMessageToChannel(ctx, botToken, channelID, message, threadTs)
}

// UpdateChatFeedback updates the feedback for a chat.
func (s *ChatService) UpdateChatFeedback(ctx context.Context, orgID, chatID uuid.UUID, feedback int8) error {
	if err := s.store.UpdateChatFeedback(ctx, chatID, feedback, orgID); err != nil {
		return fmt.Errorf("failed to update chat feedback: %w", err)
	}
	return nil
}

// GetChatbotService returns the ChatbotService instance used by this ChatService.
// This is mainly for debugging purposes.
func (s *ChatService) GetChatbotService() *ChatbotService {
	return s.chatbotService
}

// GetOrgIDForChatbot retrieves the organization ID for a given chatbotID.
func (s *ChatService) GetOrgIDForChatbot(ctx context.Context, chatbotID uuid.UUID) (uuid.UUID, error) {
	// Use GetChatbotByIDOnly which doesn't require an organization ID - better for webhook scenarios
	chatbot, err := s.store.GetChatbotByIDOnly(ctx, chatbotID)
	if err != nil {
		// Corrected to use package-level store.ErrNotFound
		if errors.Is(err, store.ErrNotFound) {
			return uuid.Nil, fmt.Errorf("chatbot with ID %s not found: %w", chatbotID, err)
		}
		return uuid.Nil, fmt.Errorf("failed to get chatbot %s from store: %w", chatbotID, err)
	}
	// If err is nil, chatbot is considered valid. The check for store.ErrNotFound handles not found.

	if chatbot.OrganizationID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("chatbot %s has a nil OrganizationID", chatbotID)
	}
	return chatbot.OrganizationID, nil
}

// FindOrCreateChatForExternalID finds a chat by its externalID and chatbotID,
// or creates a new one if not found. The provided initialMessage is added to the
// chat session (either to the existing one or as the first message in a new one).
func (s *ChatService) FindOrCreateChatForExternalID(
	ctx context.Context,
	orgID uuid.UUID, // Organization ID, assumed to be validated by the caller
	chatbotID uuid.UUID,
	externalChatID string,
	initialMessage models.Message, // The user's message from the external event
	configuration json.RawMessage, // Optional configuration for the chat
) (*models.Chat, error) {
	// Attempt to find an existing chat
	// Assuming GetChatByExternalID needs (ctx, externalID, interfaceID, orgID)
	// Passing uuid.Nil for interfaceID as a placeholder. This needs proper handling.
	existingChat, err := s.store.GetChatByExternalID(ctx, externalChatID, uuid.Nil /*TODO: interfaceID*/, orgID)

	if err == nil && existingChat != nil {
		fmt.Printf("DEBUG - ChatService.FindOrCreateChatForExternalID: Found existing chat ID %s for externalID %s. Adding message.\n", existingChat.ID, externalChatID)

		// If configuration is provided, update the chat configuration
		if len(configuration) > 0 && !bytes.Equal(configuration, []byte("{}")) && !bytes.Equal(configuration, []byte("null")) {
			existingChat.Configuration = configuration
			// Update the configuration in the database
			err := s.store.UpdateChatConfiguration(ctx, existingChat.ID, configuration, existingChat.OrganizationID)
			if err != nil {
				fmt.Printf("WARNING - ChatService.FindOrCreateChatForExternalID: Failed to update chat configuration: %v\n", err)
				// Continue processing even if this fails
			} else {
				fmt.Printf("DEBUG - ChatService.FindOrCreateChatForExternalID: Updated chat configuration for chat ID %s\n", existingChat.ID)
			}
		}

		// AddMessageToChat expects string content and returns *models.ChatResponse.
		// We pass initialMessage.Content and then re-fetch the *models.Chat.
		_, addMsgErr := s.AddMessageToChat(ctx, existingChat.OrganizationID, existingChat.ID, initialMessage.Content)
		if addMsgErr != nil {
			return nil, fmt.Errorf("failed to add message (content) to existing chat %s for externalID %s: %w", existingChat.ID, externalChatID, addMsgErr)
		}
		// Re-fetch to get *models.Chat object as AddMessageToChat returns ChatResponse
		updatedDbChat, getErr := s.store.GetChatByID(ctx, existingChat.ID, existingChat.OrganizationID)
		if getErr != nil {
			return nil, fmt.Errorf("failed to re-fetch chat %s after adding message: %w", existingChat.ID, getErr)
		}
		return updatedDbChat, nil
	}

	// Corrected to use package-level store.ErrNotFound
	if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("error checking for existing chat with externalID %s: %w", externalChatID, err)
	}

	fmt.Printf("DEBUG - ChatService.FindOrCreateChatForExternalID: No chat found for externalID %s. Creating new chat for chatbot %s, org %s.\n", externalChatID, chatbotID, orgID)

	if initialMessage.Timestamp.IsZero() {
		initialMessage.Timestamp = time.Now().UTC()
	}

	// Convert models.Message to models.ChatMessage for ChatData
	initialChatMessage := models.ChatMessage{
		Role:      initialMessage.Role,
		Content:   initialMessage.Content,
		Timestamp: initialMessage.Timestamp.Unix(), // models.ChatMessage uses Unix timestamp
		SentBy:    initialMessage.Role,             // Or derive SentBy appropriately
		Hide:      0,                               // Default
		// Metadata would need conversion if models.ChatMessage supports it
	}
	messagesForChatData := []models.ChatMessage{initialChatMessage}
	chatDataJSON, marshalErr := json.Marshal(messagesForChatData)
	if marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal initial message for new chat: %w", marshalErr)
	}

	// Use the provided configuration or default to empty JSON object
	configJSON := configuration
	if len(configJSON) == 0 {
		configJSON = []byte("{}")
	}

	newChat := &models.Chat{
		ID:             uuid.New(),
		OrganizationID: orgID,
		ChatbotID:      chatbotID,
		ExternalChatID: externalChatID, // Corrected: assign string directly
		ChatData:       chatDataJSON,   // Use marshalled ChatMessage
		Configuration:  configJSON,     // Use the provided configuration
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		Status:         "ACTIVE", // Set a default status
		// InterfaceID: uuid.Nil, // Or determine actual InterfaceID if possible
	}

	// Construct CreateChatParams for the store call
	params := store.CreateChatParams{
		ID:             newChat.ID,
		OrganizationID: newChat.OrganizationID,
		ChatbotID:      newChat.ChatbotID,
		InterfaceID:    newChat.InterfaceID, // Will be uuid.Nil if not set on newChat
		ExternalChatID: newChat.ExternalChatID,
		ChatData:       newChat.ChatData,
		Configuration:  newChat.Configuration,
	}

	createdChat, createErr := s.store.CreateChat(ctx, params)
	if createErr != nil {
		return nil, fmt.Errorf("failed to create new chat with externalID %s in store: %w", externalChatID, createErr)
	}

	fmt.Printf("DEBUG - ChatService.FindOrCreateChatForExternalID: Created new chat ID %s for externalID %s.\n", createdChat.ID, externalChatID)
	return createdChat, nil // s.store.CreateChat now returns *models.Chat, so this is fine.
}
