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

// InterfaceService defines the interface expected from the interface service.
type InterfaceService interface {
	CreateInterface(ctx context.Context, req models.CreateInterfaceRequest, orgID uuid.UUID) (*models.InterfaceResponse, error)
	GetInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*models.InterfaceResponse, error)
	ListInterfaces(ctx context.Context, orgID uuid.UUID) ([]models.InterfaceResponse, error)
	UpdateInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID, req models.CreateInterfaceRequest) (*models.InterfaceResponse, error)
	DeleteInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
}

type InterfaceHandler struct {
	intfService InterfaceService
}

func NewInterfaceHandler(intfSvc InterfaceService) *InterfaceHandler {
	return &InterfaceHandler{
		intfService: intfSvc,
	}
}

// HandleCreateInterface handles POST /v1/interfaces
func (h *InterfaceHandler) HandleCreateInterface(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	var req models.CreateInterfaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	resp, err := h.intfService.CreateInterface(r.Context(), req, orgID)
	if err != nil {
		log.Printf("ERROR [InterfaceHandler] HandleCreateInterface for OrgID %s: %v", orgID, err)
		switch {
		case errors.Is(err, services.ErrInterfaceValidation):
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, services.ErrInterfaceCredentialMismatch):
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
		case err.Error() == fmt.Sprintf("interface with name '%s' already exists in this organization", req.Name):
			httputil.RespondError(w, http.StatusConflict, err.Error())
		default:
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create interface")
		}
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, resp)
}

// HandleListInterfaces handles GET /v1/interfaces
func (h *InterfaceHandler) HandleListInterfaces(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	interfaces, err := h.intfService.ListInterfaces(r.Context(), orgID)
	if err != nil {
		log.Printf("ERROR [InterfaceHandler] HandleListInterfaces for OrgID %s: %v", orgID, err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to list interfaces")
		return
	}

	if interfaces == nil {
		interfaces = []models.InterfaceResponse{}
	}
	httputil.RespondJSON(w, http.StatusOK, interfaces)
}

// HandleGetInterface handles GET /v1/interfaces/{interfaceID}
func (h *InterfaceHandler) HandleGetInterface(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	intfIDStr := chi.URLParam(r, "interfaceID")
	intfID, err := uuid.Parse(intfIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid interface ID format")
		return
	}

	resp, err := h.intfService.GetInterface(r.Context(), intfID, orgID)
	if err != nil {
		log.Printf("ERROR [InterfaceHandler] HandleGetInterface for ID %s, OrgID %s: %v", intfID, orgID, err)
		if errors.Is(err, services.ErrInterfaceNotFound) {
			httputil.RespondError(w, http.StatusNotFound, err.Error())
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get interface")
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, resp)
}

// HandleUpdateInterface handles PUT /v1/interfaces/{interfaceID}
func (h *InterfaceHandler) HandleUpdateInterface(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	intfIDStr := chi.URLParam(r, "interfaceID")
	intfID, err := uuid.Parse(intfIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid interface ID format")
		return
	}

	var req models.CreateInterfaceRequest // Reuse Create request DTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	resp, err := h.intfService.UpdateInterface(r.Context(), intfID, orgID, req)
	if err != nil {
		log.Printf("ERROR [InterfaceHandler] HandleUpdateInterface for ID %s, OrgID %s: %v", intfID, orgID, err)
		switch {
		case errors.Is(err, services.ErrInterfaceNotFound):
			httputil.RespondError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, services.ErrInterfaceValidation):
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, services.ErrInterfaceCredentialMismatch):
			httputil.RespondError(w, http.StatusBadRequest, "Credential update not supported or invalid")
		case err.Error() == fmt.Sprintf("interface with name '%s' already exists in this organization", req.Name):
			httputil.RespondError(w, http.StatusConflict, err.Error())
		default:
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to update interface")
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, resp)
}

// HandleDeleteInterface handles DELETE /v1/interfaces/{interfaceID}
func (h *InterfaceHandler) HandleDeleteInterface(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	intfIDStr := chi.URLParam(r, "interfaceID")
	intfID, err := uuid.Parse(intfIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid interface ID format")
		return
	}

	err = h.intfService.DeleteInterface(r.Context(), intfID, orgID)
	if err != nil {
		log.Printf("ERROR [InterfaceHandler] HandleDeleteInterface for ID %s, OrgID %s: %v", intfID, orgID, err)
		if errors.Is(err, services.ErrInterfaceNotFound) {
			httputil.RespondError(w, http.StatusNotFound, err.Error())
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to delete interface")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
