package services

import (
	"buildmychat-backend/internal/models"
	"buildmychat-backend/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ChatService handles chat-related business logic.
type ChatService struct {
	store          store.Store
	chatbotService *ChatbotService
}

// NewChatService creates a new ChatService.
func NewChatService(store store.Store, chatbotService *ChatbotService) *ChatService {
	return &ChatService{
		store:          store,
		chatbotService: chatbotService,
	}
}

// mapChatToResponse converts a DB chat model to an API response DTO.
func (s *ChatService) mapChatToResponse(ctx context.Context, dbChat *models.Chat, includeChatbot bool) (*models.ChatResponse, error) {
	// Parse chat data as messages
	var messages []models.ChatMessage
	if err := json.Unmarshal(dbChat.ChatData, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse chat data: %w", err)
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
	// Validate required fields
	if req.ChatbotID == uuid.Nil {
		return nil, fmt.Errorf("chatbot_id is required")
	}

	// If interface ID is not provided, we need to select a default one
	interfaceID := uuid.Nil
	if req.InterfaceID != nil {
		interfaceID = *req.InterfaceID
	} else {
		// Get associated interfaces for this chatbot to find a default
		mappings, err := s.store.GetChatbotMappings(ctx, req.ChatbotID, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to get chatbot mappings: %w", err)
		}

		// Use the first interface if available
		if len(mappings.Interfaces) > 0 {
			interfaceID = mappings.Interfaces[0].ID
		} else {
			return nil, fmt.Errorf("chatbot has no associated interfaces and none was provided")
		}
	}

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

	// Create the chat in the database
	dbChat := &models.Chat{
		ID:             uuid.New(), // Generate a new UUID
		ChatbotID:      req.ChatbotID,
		OrganizationID: orgID,
		InterfaceID:    interfaceID,
		ExternalChatID: externalChatID,
		ChatData:       messagesJSON,
		Status:         "ACTIVE",
	}

	// Create the chat in the database
	params := store.CreateChatParams{
		ID:             dbChat.ID,
		OrganizationID: orgID,
		ChatbotID:      req.ChatbotID,
		InterfaceID:    interfaceID,
		ExternalChatID: externalChatID,
		ChatData:       messagesJSON,
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
func (s *ChatService) AddAssistantMessageToChat(ctx context.Context, orgID, chatID uuid.UUID, message string, metadata *json.RawMessage) (*models.ChatResponse, error) {
	// First get the chat to ensure it exists and belongs to the organization
	_, err := s.store.GetChatByID(ctx, chatID, orgID)
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

// UpdateChatFeedback updates the feedback for a chat.
func (s *ChatService) UpdateChatFeedback(ctx context.Context, orgID, chatID uuid.UUID, feedback int8) error {
	if err := s.store.UpdateChatFeedback(ctx, chatID, feedback, orgID); err != nil {
		return fmt.Errorf("failed to update chat feedback: %w", err)
	}
	return nil
}
