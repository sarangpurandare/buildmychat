package handlers

import (
	"buildmychat-backend/internal/auth"
	"buildmychat-backend/internal/models"
	"buildmychat-backend/internal/services"
	"buildmychat-backend/pkg/httputil"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// KBService defines the interface expected from the knowledge base service.
type KBService interface {
	CreateKnowledgeBase(ctx context.Context, req models.CreateKnowledgeBaseRequest, orgID uuid.UUID) (*models.KnowledgeBaseResponse, error)
	GetKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*models.KnowledgeBaseResponse, error)
	ListKnowledgeBases(ctx context.Context, orgID uuid.UUID) ([]models.KnowledgeBaseResponse, error)
	UpdateKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID, req models.CreateKnowledgeBaseRequest) (*models.KnowledgeBaseResponse, error)
	DeleteKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
}

type KBHandler struct {
	kbService KBService
}

func NewKBHandler(kbSvc KBService) *KBHandler {
	return &KBHandler{
		kbService: kbSvc,
	}
}

// HandleCreateKnowledgeBase handles POST /v1/knowledge-bases
func (h *KBHandler) HandleCreateKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	var req models.CreateKnowledgeBaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	resp, err := h.kbService.CreateKnowledgeBase(r.Context(), req, orgID)
	if err != nil {
		log.Printf("ERROR [KBHandler] HandleCreateKB for OrgID %s: %v", orgID, err)
		switch {
		case errors.Is(err, services.ErrKBValidation):
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, services.ErrKBCredentialMismatch):
			httputil.RespondError(w, http.StatusBadRequest, err.Error()) // 400 Bad Request - wrong cred type
		case err.Error() == fmt.Sprintf("knowledge base with name '%s' already exists in this organization", req.Name):
			httputil.RespondError(w, http.StatusConflict, err.Error()) // 409 Conflict
		default:
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create knowledge base")
		}
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, resp)
}

// HandleListKnowledgeBases handles GET /v1/knowledge-bases
func (h *KBHandler) HandleListKnowledgeBases(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	kbs, err := h.kbService.ListKnowledgeBases(r.Context(), orgID)
	if err != nil {
		log.Printf("ERROR [KBHandler] HandleListKBs for OrgID %s: %v", orgID, err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to list knowledge bases")
		return
	}

	if kbs == nil {
		kbs = []models.KnowledgeBaseResponse{}
	}
	httputil.RespondJSON(w, http.StatusOK, kbs)
}

// HandleGetKnowledgeBase handles GET /v1/knowledge-bases/{kbID}
func (h *KBHandler) HandleGetKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	kbIDStr := chi.URLParam(r, "kbID")
	kbID, err := uuid.Parse(kbIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid knowledge base ID format")
		return
	}

	resp, err := h.kbService.GetKnowledgeBase(r.Context(), kbID, orgID)
	if err != nil {
		log.Printf("ERROR [KBHandler] HandleGetKB for ID %s, OrgID %s: %v", kbID, orgID, err)
		if errors.Is(err, services.ErrKBNotFound) {
			httputil.RespondError(w, http.StatusNotFound, err.Error())
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get knowledge base")
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, resp)
}

// HandleUpdateKnowledgeBase handles PUT /v1/knowledge-bases/{kbID}
// Note: Using PUT implies replacing the resource or creating if not exists.
// PATCH might be more appropriate for partial updates, but using PUT with CreateKnowledgeBaseRequest for simplicity.
func (h *KBHandler) HandleUpdateKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	kbIDStr := chi.URLParam(r, "kbID")
	kbID, err := uuid.Parse(kbIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid knowledge base ID format")
		return
	}

	var req models.CreateKnowledgeBaseRequest // Reuse Create request DTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	resp, err := h.kbService.UpdateKnowledgeBase(r.Context(), kbID, orgID, req)
	if err != nil {
		log.Printf("ERROR [KBHandler] HandleUpdateKB for ID %s, OrgID %s: %v", kbID, orgID, err)
		switch {
		case errors.Is(err, services.ErrKBNotFound):
			httputil.RespondError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, services.ErrKBValidation):
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, services.ErrKBCredentialMismatch):
			// This error shouldn't happen if KB update doesn't allow changing credential ID
			httputil.RespondError(w, http.StatusBadRequest, "Credential update not supported or invalid")
		case err.Error() == "knowledge base name conflicts with an existing one in this organization":
			httputil.RespondError(w, http.StatusConflict, err.Error())
		default:
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to update knowledge base")
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, resp)
}

// HandleDeleteKnowledgeBase handles DELETE /v1/knowledge-bases/{kbID}
func (h *KBHandler) HandleDeleteKnowledgeBase(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	kbIDStr := chi.URLParam(r, "kbID")
	kbID, err := uuid.Parse(kbIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid knowledge base ID format")
		return
	}

	err = h.kbService.DeleteKnowledgeBase(r.Context(), kbID, orgID)
	if err != nil {
		log.Printf("ERROR [KBHandler] HandleDeleteKB for ID %s, OrgID %s: %v", kbID, orgID, err)
		if errors.Is(err, services.ErrKBNotFound) {
			httputil.RespondError(w, http.StatusNotFound, err.Error())
		} else {
			// Consider specific error for FK violation (e.g., 409 Conflict)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to delete knowledge base")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
