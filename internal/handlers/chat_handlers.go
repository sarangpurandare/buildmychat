package handlers

import (
	"buildmychat-backend/internal/models"
	"buildmychat-backend/internal/services"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ChatHandlers handles HTTP requests related to chats.
type ChatHandlers struct {
	chatService *services.ChatService
}

// NewChatHandlers creates a new ChatHandlers instance.
func NewChatHandlers(chatService *services.ChatService) *ChatHandlers {
	return &ChatHandlers{
		chatService: chatService,
	}
}

// HandleCreateChat handles requests to create a new chat.
func (h *ChatHandlers) HandleCreateChat(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, err := GetOrgIDFromContext(r.Context())
	fmt.Println("DEBUG - HandleCreateChat - orgID from context:", orgID)
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// DEBUG: List all chatbots for this organization
	h.debugListAllChatbotsForOrg(r.Context(), orgID)

	// Parse request body
	var req models.CreateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	fmt.Println("DEBUG - HandleCreateChat - request ChatbotID:", req.ChatbotID)
	if req.InterfaceID != nil {
		fmt.Println("DEBUG - HandleCreateChat - request InterfaceID:", *req.InterfaceID)
	} else {
		fmt.Println("DEBUG - HandleCreateChat - request InterfaceID: nil")
	}

	// Call service to create chat
	chat, err := h.chatService.CreateChat(r.Context(), orgID, req)
	if err != nil {
		fmt.Println("DEBUG - HandleCreateChat - Error:", err)
		RespondWithError(w, http.StatusInternalServerError, "Failed to create chat: "+err.Error())
		return
	}

	// Respond with created chat
	RespondWithJSON(w, http.StatusCreated, chat)
}

// HandleGetChatByID handles requests to get a chat by ID.
func (h *ChatHandlers) HandleGetChatByID(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, err := GetOrgIDFromContext(r.Context())
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Extract chat ID from URL
	chatIDStr := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid chat ID")
		return
	}

	// Call service to get chat
	chat, err := h.chatService.GetChatByID(r.Context(), orgID, chatID, true)
	if err != nil {
		if errors.Is(err, errors.New("record not found")) {
			RespondWithError(w, http.StatusNotFound, "Chat not found")
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "Failed to get chat: "+err.Error())
		return
	}

	// Respond with chat
	RespondWithJSON(w, http.StatusOK, chat)
}

// HandleAddUserMessage handles requests to add a user message to a chat.
func (h *ChatHandlers) HandleAddUserMessage(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, err := GetOrgIDFromContext(r.Context())
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Extract chat ID from URL
	chatIDStr := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid chat ID")
		return
	}

	// Parse request body
	var req models.AddMessageAsUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Call service to add message
	updatedChat, err := h.chatService.AddMessageToChat(r.Context(), orgID, chatID, req.Message)
	if err != nil {
		if errors.Is(err, errors.New("record not found")) {
			RespondWithError(w, http.StatusNotFound, "Chat not found")
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "Failed to add message: "+err.Error())
		return
	}

	// Respond with updated chat
	RespondWithJSON(w, http.StatusOK, updatedChat)
}

// HandleAddAssistantMessage handles requests to add an assistant message to a chat.
func (h *ChatHandlers) HandleAddAssistantMessage(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, err := GetOrgIDFromContext(r.Context())
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Extract chat ID from URL
	chatIDStr := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid chat ID")
		return
	}

	// Parse request body
	var req models.AddMessageAsAssistantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get the chat first to check if it exists and get its details
	chat, err := h.chatService.GetChatByID(r.Context(), orgID, chatID, false)
	if err != nil {
		if errors.Is(err, errors.New("record not found")) {
			RespondWithError(w, http.StatusNotFound, "Chat not found")
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "Failed to get chat: "+err.Error())
		return
	}

	// Determine if we should send to interface
	sendToInterface := false
	if req.SendToInterface != nil {
		sendToInterface = *req.SendToInterface
	}

	// Call service to add assistant message
	updatedChat, err := h.chatService.AddAssistantMessageToChat(r.Context(), orgID, chatID, req.Message, req.Metadata)
	if err != nil {
		if errors.Is(err, errors.New("record not found")) {
			RespondWithError(w, http.StatusNotFound, "Chat not found")
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "Failed to add assistant message: "+err.Error())
		return
	}

	// If sendToInterface is true and the chat has an associated interface, send the message
	if sendToInterface && chat.InterfaceID != uuid.Nil {
		if err := h.chatService.SendMessageToInterface(r.Context(), orgID, chatID, req.Message); err != nil {
			// Log the error but don't fail the entire operation
			fmt.Printf("WARNING - HandleAddAssistantMessage - Failed to send message to interface: %v\n", err)
		}
	}

	// Respond with updated chat
	RespondWithJSON(w, http.StatusOK, updatedChat)
}

// HandleListChats handles requests to list chats for the organization or chatbot.
func (h *ChatHandlers) HandleListChats(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, err := GetOrgIDFromContext(r.Context())
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Extract query parameters
	chatbotIDStr := r.URL.Query().Get("chatbot_id")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	//adding the query parameters to the log, will add code later to parse the query parameters
	fmt.Println("chatbotIDStr", chatbotIDStr)
	fmt.Println("limitStr", limitStr)
	fmt.Println("offsetStr", offsetStr)
	// Parse and set defaults for limit and offset
	limit := 20
	offset := 0
	// ... (code to parse query parameters omitted for brevity)

	var chats *models.ListChatsResponse
	if chatbotIDStr != "" {
		// If chatbot ID is provided, list chats for that chatbot
		chatbotID, err := uuid.Parse(chatbotIDStr)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid chatbot ID")
			return
		}
		chats, err = h.chatService.ListChatsByChatbot(r.Context(), orgID, chatbotID, limit, offset, true)
	} else {
		// Otherwise, list all chats for the organization
		chats, err = h.chatService.ListChatsByOrg(r.Context(), orgID, limit, offset, true)
	}

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to list chats: "+err.Error())
		return
	}

	// Respond with chats
	RespondWithJSON(w, http.StatusOK, chats)
}

func (h *ChatHandlers) debugListAllChatbotsForOrg(ctx context.Context, orgID uuid.UUID) {
	// Try to access chatbot service through chat service
	chatbotService := h.chatService.GetChatbotService()
	if chatbotService == nil {
		fmt.Println("DEBUG - Cannot debug list chatbots: ChatbotService not accessible")
		return
	}

	response, err := chatbotService.ListChatbots(ctx, orgID)
	if err != nil {
		fmt.Printf("DEBUG - Error listing chatbots for orgID %s: %v\n", orgID, err)
		return
	}

	// Print information about each chatbot
	fmt.Printf("DEBUG - Found %d chatbots for organization %s:\n", len(response.Chatbots), orgID)
	for i, chatbot := range response.Chatbots {
		fmt.Printf("  %d. ID: %s, Name: %s, OrgID: %s\n",
			i+1, chatbot.ID, chatbot.Name, chatbot.OrganizationID)

		// Print interfaces if any
		if len(chatbot.Interfaces) > 0 {
			fmt.Printf("     Interfaces (%d):\n", len(chatbot.Interfaces))
			for j, iface := range chatbot.Interfaces {
				fmt.Printf("       %d. ID: %s, Name: %s\n", j+1, iface.ID, iface.Name)
			}
		} else {
			fmt.Println("     No interfaces")
		}
	}

	if len(response.Chatbots) == 0 {
		fmt.Println("DEBUG - No chatbots found for this organization")
	}
}
