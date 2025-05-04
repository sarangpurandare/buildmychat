package handlers

import (
	"buildmychat-backend/internal/auth"
	"buildmychat-backend/internal/models"
	"buildmychat-backend/internal/services"
	"buildmychat-backend/pkg/httputil"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// CredentialsService defines the interface expected from the credentials service.
type CredentialsService interface {
	CreateCredential(ctx context.Context, req models.CreateCredentialRequest, orgID uuid.UUID) (*models.CredentialResponse, error)
	GetCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*models.CredentialResponse, error)
	ListCredentials(ctx context.Context, orgID uuid.UUID, serviceType *string) ([]models.CredentialResponse, error)
	DeleteCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
	TestCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*models.TestCredentialResponse, error)
}

type CredentialsHandler struct {
	credService CredentialsService
}

func NewCredentialsHandler(credSvc CredentialsService) *CredentialsHandler {
	return &CredentialsHandler{
		credService: credSvc,
	}
}

// HandleCreateCredential handles POST /v1/credentials
func (h *CredentialsHandler) HandleCreateCredential(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	var req models.CreateCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	// Basic validation (Name is optional now)
	if req.ServiceType == "" || len(req.Credentials) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "Missing required fields: service_type, credentials")
		return
	}

	resp, err := h.credService.CreateCredential(r.Context(), req, orgID)
	if err != nil {
		log.Printf("ERROR [CredHandler] HandleCreateCredential for OrgID %s: %v", orgID, err)
		if errors.Is(err, services.ErrCredentialValidation) {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
		} else if errors.Is(err, services.ErrCredentialEncryption) {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to secure credentials")
		} else if errors.Is(err, services.ErrCredentialTestFailed) { // Handle pre-save test failure
			httputil.RespondError(w, http.StatusBadRequest, err.Error()) // Return as Bad Request
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create credential")
		}
		return
	}

	httputil.RespondJSON(w, http.StatusCreated, resp)
}

// HandleListCredentials handles GET /v1/credentials
func (h *CredentialsHandler) HandleListCredentials(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	// Optional filtering by query parameter
	serviceTypeQuery := r.URL.Query().Get("service_type")
	var serviceTypeFilter *string
	if serviceTypeQuery != "" {
		// TODO: Validate serviceTypeQuery against known types?
		serviceTypeFilter = &serviceTypeQuery
	}

	creds, err := h.credService.ListCredentials(r.Context(), orgID, serviceTypeFilter)
	if err != nil {
		log.Printf("ERROR [CredHandler] HandleListCredentials for OrgID %s: %v", orgID, err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to list credentials")
		return
	}

	// Return empty list if no credentials found, not an error
	if creds == nil {
		creds = []models.CredentialResponse{}
	}

	httputil.RespondJSON(w, http.StatusOK, creds)
}

// HandleGetCredential handles GET /v1/credentials/{credentialID}
func (h *CredentialsHandler) HandleGetCredential(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	credIDStr := chi.URLParam(r, "credentialID")
	credID, err := uuid.Parse(credIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid credential ID format")
		return
	}

	resp, err := h.credService.GetCredential(r.Context(), credID, orgID)
	if err != nil {
		log.Printf("ERROR [CredHandler] HandleGetCredential for ID %s, OrgID %s: %v", credID, orgID, err)
		if errors.Is(err, services.ErrCredentialNotFound) {
			httputil.RespondError(w, http.StatusNotFound, err.Error())
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get credential")
		}
		return
	}

	httputil.RespondJSON(w, http.StatusOK, resp)
}

// HandleDeleteCredential handles DELETE /v1/credentials/{credentialID}
func (h *CredentialsHandler) HandleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	credIDStr := chi.URLParam(r, "credentialID")
	credID, err := uuid.Parse(credIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid credential ID format")
		return
	}

	err = h.credService.DeleteCredential(r.Context(), credID, orgID)
	if err != nil {
		log.Printf("ERROR [CredHandler] HandleDeleteCredential for ID %s, OrgID %s: %v", credID, orgID, err)
		if errors.Is(err, services.ErrCredentialNotFound) {
			httputil.RespondError(w, http.StatusNotFound, err.Error())
		} else if errors.Is(err, services.ErrCredentialInUse) {
			httputil.RespondError(w, http.StatusConflict, err.Error()) // 409 Conflict is suitable
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to delete credential")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent) // 204 No Content on successful deletion
}

// HandleTestCredential handles POST /v1/credentials/{credentialID}/test
func (h *CredentialsHandler) HandleTestCredential(w http.ResponseWriter, r *http.Request) {
	orgID, ok := auth.GetOrgIDFromContext(r.Context())
	if !ok {
		httputil.RespondError(w, http.StatusUnauthorized, "Organization ID not found in token context")
		return
	}

	credIDStr := chi.URLParam(r, "credentialID")
	credID, err := uuid.Parse(credIDStr)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid credential ID format")
		return
	}

	resp, err := h.credService.TestCredential(r.Context(), credID, orgID)
	if err != nil {
		log.Printf("ERROR [CredHandler] HandleTestCredential for ID %s, OrgID %s: %v", credID, orgID, err)
		// TestCredential service method returns the response payload even on logical failure
		if errors.Is(err, services.ErrCredentialNotFound) {
			httputil.RespondError(w, http.StatusNotFound, err.Error())
		} else if errors.Is(err, services.ErrCredentialDecryption) {
			httputil.RespondJSON(w, http.StatusInternalServerError, resp) // Use payload from service
		} else if errors.Is(err, services.ErrCredentialTestFailed) {
			httputil.RespondJSON(w, http.StatusOK, resp) // Test failed logically, but request OK (200)
		} else if errors.Is(err, services.ErrUnsupportedServiceType) {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to test credential")
		}
		return
	}

	// Test succeeded
	httputil.RespondJSON(w, http.StatusOK, resp)
}
