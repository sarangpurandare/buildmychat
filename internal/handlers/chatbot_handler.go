package handlers

import (
	"buildmychat-backend/internal/auth"
	"buildmychat-backend/internal/models"
	"buildmychat-backend/internal/services"
	"buildmychat-backend/internal/store"
	"buildmychat-backend/pkg/httputil"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ChatbotHandlers holds the dependencies for chatbot handlers.
type ChatbotHandlers struct {
	Service *services.ChatbotService
}

// NewChatbotHandlers creates a new ChatbotHandlers.
func NewChatbotHandlers(cs *services.ChatbotService) *ChatbotHandlers {
	return &ChatbotHandlers{Service: cs}
}

// CreateChatbot handles the creation of a new chatbot.
// POST /api/v1/chatbots
func (h *ChatbotHandlers) CreateChatbot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	var req models.CreateChatbotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Basic validation (can be expanded)
	// if req.Name == nil || *req.Name == "" {
	// 	 httputil.RespondError(w, http.StatusBadRequest, "Chatbot name is required")
	// 	 return
	// }

	chatbotResp, err := h.Service.CreateChatbot(ctx, orgID, req)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create chatbot: %v", err))
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, chatbotResp)
}

// ListChatbots handles listing all chatbots for the organization.
// GET /api/v1/chatbots
func (h *ChatbotHandlers) ListChatbots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	chatbotsResp, err := h.Service.ListChatbots(ctx, orgID)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list chatbots: %v", err))
		return
	}

	httputil.RespondJSON(w, http.StatusOK, chatbotsResp)
}

// GetChatbotByID handles retrieving a specific chatbot.
// GET /api/v1/chatbots/{chatbotID}
func (h *ChatbotHandlers) GetChatbotByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	chatbotIDStr := chi.URLParam(r, "chatbotID")
	chatbotID, err := uuid.Parse(chatbotIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid chatbot ID format")
		return
	}

	chatbotResp, err := h.Service.GetChatbotByID(ctx, orgID, chatbotID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "Chatbot not found")
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get chatbot: %v", err))
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, chatbotResp)
}

// UpdateChatbot handles updating a specific chatbot.
// PUT /api/v1/chatbots/{chatbotID}
func (h *ChatbotHandlers) UpdateChatbot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	chatbotIDStr := chi.URLParam(r, "chatbotID")
	chatbotID, err := uuid.Parse(chatbotIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid chatbot ID format")
		return
	}

	var req models.UpdateChatbotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Add validation if necessary (e.g., ensure at least one field is being updated)
	if req.Name == nil && req.SystemPrompt == nil && req.LLMModel == nil && req.Configuration == nil {
		httputil.RespondError(w, http.StatusBadRequest, "No update fields provided")
		return
	}

	updatedChatbot, err := h.Service.UpdateChatbot(ctx, orgID, chatbotID, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "Chatbot not found")
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update chatbot: %v", err))
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, updatedChatbot)
}

// UpdateChatbotStatus handles activating or deactivating a chatbot.
// PATCH /api/v1/chatbots/{chatbotID}/status
func (h *ChatbotHandlers) UpdateChatbotStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	chatbotIDStr := chi.URLParam(r, "chatbotID")
	chatbotID, err := uuid.Parse(chatbotIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid chatbot ID format")
		return
	}

	var req models.UpdateChatbotStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	err = h.Service.UpdateChatbotStatus(ctx, orgID, chatbotID, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "Chatbot not found")
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update chatbot status: %v", err))
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Chatbot status updated successfully"})
}

// DeleteChatbot handles deleting a specific chatbot.
// DELETE /api/v1/chatbots/{chatbotID}
func (h *ChatbotHandlers) DeleteChatbot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	chatbotIDStr := chi.URLParam(r, "chatbotID")
	chatbotID, err := uuid.Parse(chatbotIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid chatbot ID format")
		return
	}

	err = h.Service.DeleteChatbot(ctx, orgID, chatbotID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "Chatbot not found")
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete chatbot: %v", err))
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Chatbot deleted successfully"})
}

// GetChatbotMappings handles retrieving all mappings for a chatbot.
// GET /api/v1/chatbots/{chatbotID}/mappings
func (h *ChatbotHandlers) GetChatbotMappings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	chatbotIDStr := chi.URLParam(r, "chatbotID")
	chatbotID, err := uuid.Parse(chatbotIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid chatbot ID format")
		return
	}

	mappings, err := h.Service.GetChatbotMappings(ctx, orgID, chatbotID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "Chatbot not found")
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get chatbot mappings: %v", err))
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, mappings)
}

// AddKnowledgeBase handles adding a knowledge base to a chatbot.
// POST /api/v1/chatbots/{chatbotID}/knowledge-bases
func (h *ChatbotHandlers) AddKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	chatbotIDStr := chi.URLParam(r, "chatbotID")
	chatbotID, err := uuid.Parse(chatbotIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid chatbot ID format")
		return
	}

	var req models.AddKnowledgeBaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if req.KBID == uuid.Nil {
		httputil.RespondError(w, http.StatusBadRequest, "Knowledge base ID is required")
		return
	}

	err = h.Service.AddKnowledgeBase(ctx, orgID, chatbotID, req.KBID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "Chatbot or knowledge base not found")
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add knowledge base: %v", err))
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Knowledge base added successfully"})
}

// RemoveKnowledgeBase handles removing a knowledge base from a chatbot.
// DELETE /api/v1/chatbots/{chatbotID}/knowledge-bases/{kbID}
func (h *ChatbotHandlers) RemoveKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	chatbotIDStr := chi.URLParam(r, "chatbotID")
	chatbotID, err := uuid.Parse(chatbotIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid chatbot ID format")
		return
	}

	kbIDStr := chi.URLParam(r, "kbID")
	kbID, err := uuid.Parse(kbIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid knowledge base ID format")
		return
	}

	err = h.Service.RemoveKnowledgeBase(ctx, orgID, chatbotID, kbID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "Mapping not found")
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to remove knowledge base: %v", err))
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Knowledge base removed successfully"})
}

// AddInterface handles adding an interface to a chatbot.
// POST /api/v1/chatbots/{chatbotID}/interfaces
func (h *ChatbotHandlers) AddInterface(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	chatbotIDStr := chi.URLParam(r, "chatbotID")
	chatbotID, err := uuid.Parse(chatbotIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid chatbot ID format")
		return
	}

	var req models.AddInterfaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if req.InterfaceID == uuid.Nil {
		httputil.RespondError(w, http.StatusBadRequest, "Interface ID is required")
		return
	}

	err = h.Service.AddInterface(ctx, orgID, chatbotID, req.InterfaceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "Chatbot or interface not found")
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add interface: %v", err))
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Interface added successfully"})
}

// RemoveInterface handles removing an interface from a chatbot.
// DELETE /api/v1/chatbots/{chatbotID}/interfaces/{interfaceID}
func (h *ChatbotHandlers) RemoveInterface(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in context")
		return
	}

	chatbotIDStr := chi.URLParam(r, "chatbotID")
	chatbotID, err := uuid.Parse(chatbotIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid chatbot ID format")
		return
	}

	interfaceIDStr := chi.URLParam(r, "interfaceID")
	interfaceID, err := uuid.Parse(interfaceIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid interface ID format")
		return
	}

	err = h.Service.RemoveInterface(ctx, orgID, chatbotID, interfaceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.RespondError(w, http.StatusNotFound, "Mapping not found")
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to remove interface: %v", err))
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Interface removed successfully"})
}
