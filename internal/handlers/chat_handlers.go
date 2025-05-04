package handlers

import (
	"buildmychat-backend/internal/models"
	"buildmychat-backend/internal/services"
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
	orgID, err := getOrgIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse request body
	var req models.CreateChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Call service to create chat
	chat, err := h.chatService.CreateChat(r.Context(), orgID, req)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create chat: "+err.Error())
		return
	}

	// Respond with created chat
	respondWithJSON(w, http.StatusCreated, chat)
}

// HandleGetChatByID handles requests to get a chat by ID.
func (h *ChatHandlers) HandleGetChatByID(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, err := getOrgIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Extract chat ID from URL
	chatIDStr := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid chat ID")
		return
	}

	// Call service to get chat
	chat, err := h.chatService.GetChatByID(r.Context(), orgID, chatID, true)
	if err != nil {
		if errors.Is(err, errors.New("record not found")) {
			respondWithError(w, http.StatusNotFound, "Chat not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to get chat: "+err.Error())
		return
	}

	// Respond with chat
	respondWithJSON(w, http.StatusOK, chat)
}

// HandleAddUserMessage handles requests to add a user message to a chat.
func (h *ChatHandlers) HandleAddUserMessage(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, err := getOrgIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Extract chat ID from URL
	chatIDStr := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid chat ID")
		return
	}

	// Parse request body
	var req models.AddMessageAsUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Call service to add message
	updatedChat, err := h.chatService.AddMessageToChat(r.Context(), orgID, chatID, req.Message)
	if err != nil {
		if errors.Is(err, errors.New("record not found")) {
			respondWithError(w, http.StatusNotFound, "Chat not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to add message: "+err.Error())
		return
	}

	// Respond with updated chat
	respondWithJSON(w, http.StatusOK, updatedChat)
}

// HandleAddAssistantMessage handles requests to add an assistant message to a chat.
func (h *ChatHandlers) HandleAddAssistantMessage(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, err := getOrgIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Extract chat ID from URL
	chatIDStr := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid chat ID")
		return
	}

	// Parse request body
	var req models.AddMessageAsAssistantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Call service to add assistant message
	updatedChat, err := h.chatService.AddAssistantMessageToChat(r.Context(), orgID, chatID, req.Message, req.Metadata)
	if err != nil {
		if errors.Is(err, errors.New("record not found")) {
			respondWithError(w, http.StatusNotFound, "Chat not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to add assistant message: "+err.Error())
		return
	}

	// Respond with updated chat
	respondWithJSON(w, http.StatusOK, updatedChat)
}

// HandleListChats handles requests to list chats for the organization or chatbot.
func (h *ChatHandlers) HandleListChats(w http.ResponseWriter, r *http.Request) {
	// Extract organization ID from context
	orgID, err := getOrgIDFromContext(r.Context())
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
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
			respondWithError(w, http.StatusBadRequest, "Invalid chatbot ID")
			return
		}
		chats, err = h.chatService.ListChatsByChatbot(r.Context(), orgID, chatbotID, limit, offset, true)
	} else {
		// Otherwise, list all chats for the organization
		chats, err = h.chatService.ListChatsByOrg(r.Context(), orgID, limit, offset, true)
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to list chats: "+err.Error())
		return
	}

	// Respond with chats
	respondWithJSON(w, http.StatusOK, chats)
}
