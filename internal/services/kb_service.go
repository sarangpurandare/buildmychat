package services

import (
	api_models "buildmychat-backend/internal/models"
	db_models "buildmychat-backend/internal/models"
	"buildmychat-backend/internal/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
)

// Custom errors for KB service
var (
	ErrKBNotFound           = errors.New("knowledge base not found")
	ErrKBValidation         = errors.New("knowledge base validation failed")
	ErrKBCredentialMismatch = errors.New("provided credential is not valid for this knowledge base type (expected NOTION)")
)

// KBService defines the interface for Knowledge Base operations.
type KBService interface {
	CreateKnowledgeBase(ctx context.Context, req api_models.CreateKnowledgeBaseRequest, orgID uuid.UUID) (*api_models.KnowledgeBaseResponse, error)
	GetKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*api_models.KnowledgeBaseResponse, error)
	ListKnowledgeBases(ctx context.Context, orgID uuid.UUID) ([]api_models.KnowledgeBaseResponse, error)
	UpdateKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID, req api_models.CreateKnowledgeBaseRequest) (*api_models.KnowledgeBaseResponse, error) // Reuse Create req for update simplicity
	DeleteKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
}

type kbService struct {
	store store.Store
}

// NewKBService creates a new KBService.
func NewKBService(s store.Store) KBService {
	return &kbService{
		store: s,
	}
}

// --- Helper Function ---
func mapDbKBToResponse(dbKB *db_models.KnowledgeBase) *api_models.KnowledgeBaseResponse {
	return &api_models.KnowledgeBaseResponse{
		ID:             dbKB.ID,
		OrganizationID: dbKB.OrganizationID,
		CredentialID:   dbKB.CredentialID,
		ServiceType:    dbKB.ServiceType,
		Name:           dbKB.Name,
		Configuration:  dbKB.Configuration,
		IsActive:       dbKB.IsActive,
		CreatedAt:      dbKB.CreatedAt,
		UpdatedAt:      dbKB.UpdatedAt,
	}
}

// CreateKnowledgeBase validates input, checks credential, and creates a new KB.
func (s *kbService) CreateKnowledgeBase(ctx context.Context, req api_models.CreateKnowledgeBaseRequest, orgID uuid.UUID) (*api_models.KnowledgeBaseResponse, error) {
	// Validate input
	if req.Name == "" {
		return nil, fmt.Errorf("%w: name cannot be empty", ErrKBValidation)
	}
	if req.CredentialID == uuid.Nil {
		return nil, fmt.Errorf("%w: credential_id cannot be empty", ErrKBValidation)
	}
	// Validate configuration JSON if provided
	if req.Configuration != nil && !json.Valid(req.Configuration) {
		return nil, fmt.Errorf("%w: configuration is not valid JSON", ErrKBValidation)
	}

	// Verify Credential exists, belongs to org, and is for NOTION
	cred, err := s.store.GetIntegrationCredentialByID(ctx, req.CredentialID, orgID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("%w: %v", ErrKBValidation, err)
		}
		log.Printf("ERROR [KBService] CreateKB: Failed GetIntegrationCredentialByID for CredID %s, OrgID %s: %v", req.CredentialID, orgID, err)
		return nil, fmt.Errorf("failed to verify credential: %w", err)
	}
	if cred.ServiceType != api_models.ServiceTypeNotion {
		return nil, ErrKBCredentialMismatch
	}

	params := store.CreateKnowledgeBaseParams{
		ID:             uuid.New(),
		OrganizationID: orgID,
		CredentialID:   req.CredentialID,
		ServiceType:    string(api_models.ServiceTypeNotion), // Hardcode for now
		Name:           req.Name,
		Configuration:  req.Configuration,
		IsActive:       false, // Default to inactive initially
	}

	dbKB, err := s.store.CreateKnowledgeBase(ctx, params)
	if err != nil {
		log.Printf("ERROR [KBService] CreateKB: Store call failed for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to save knowledge base: %w", err)
	}

	resp := mapDbKBToResponse(dbKB)
	log.Printf("[KBService] CreateKnowledgeBase: Successfully created KB ID %s for OrgID %s", resp.ID, orgID)
	return resp, nil
}

// GetKnowledgeBase retrieves a specific KB by ID for the organization.
func (s *kbService) GetKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*api_models.KnowledgeBaseResponse, error) {
	dbKB, err := s.store.GetKnowledgeBaseByID(ctx, id, orgID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrKBNotFound
		}
		log.Printf("ERROR [KBService] GetKB: Store call failed for ID %s, OrgID %s: %v", id, orgID, err)
		return nil, fmt.Errorf("failed to retrieve knowledge base: %w", err)
	}
	return mapDbKBToResponse(dbKB), nil
}

// ListKnowledgeBases retrieves all KBs for the organization.
func (s *kbService) ListKnowledgeBases(ctx context.Context, orgID uuid.UUID) ([]api_models.KnowledgeBaseResponse, error) {
	dbKBs, err := s.store.ListKnowledgeBasesByOrg(ctx, orgID)
	if err != nil {
		log.Printf("ERROR [KBService] ListKBs: Store call failed for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to list knowledge bases: %w", err)
	}

	resp := make([]api_models.KnowledgeBaseResponse, len(dbKBs))
	for i, dbKB := range dbKBs {
		kb := dbKB // Avoid loop variable capture
		resp[i] = *mapDbKBToResponse(&kb)
	}
	return resp, nil
}

// UpdateKnowledgeBase updates an existing KB.
func (s *kbService) UpdateKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID, req api_models.CreateKnowledgeBaseRequest) (*api_models.KnowledgeBaseResponse, error) {
	// Validate input that might be updated
	if req.Name != "" && len(req.Name) == 0 { // Check if Name is present but empty
		return nil, fmt.Errorf("%w: name cannot be updated to empty", ErrKBValidation)
	}
	if req.Configuration != nil && !json.Valid(req.Configuration) {
		return nil, fmt.Errorf("%w: configuration is not valid JSON", ErrKBValidation)
	}

	// Check if credential is being updated, if so, validate it
	if req.CredentialID != uuid.Nil {
		cred, err := s.store.GetIntegrationCredentialByID(ctx, req.CredentialID, orgID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, fmt.Errorf("%w: credential_id %s not found", ErrKBValidation, req.CredentialID)
			}
			log.Printf("ERROR [KBService] UpdateKB: Failed GetIntegrationCredentialByID for CredID %s, OrgID %s: %v", req.CredentialID, orgID, err)
			return nil, fmt.Errorf("failed to verify credential: %w", err)
		}
		if cred.ServiceType != api_models.ServiceTypeNotion {
			return nil, ErrKBCredentialMismatch
		}
		// Note: We are disallowing updating CredentialID via this method for simplicity.
		// If needed, a separate method or careful handling in UpdateKnowledgeBase store method is required.
		log.Printf("WARN [KBService] UpdateKB: Attempted to update CredentialID for KB %s - This is not supported via this endpoint.", id)
		// Fall through, but don't include CredentialID in the update params
	}

	params := store.UpdateKnowledgeBaseParams{
		ID:             id,
		OrganizationID: orgID,
		// Only include fields if they were provided in the request
	}
	if req.Name != "" {
		params.Name = &req.Name
	}
	if req.Configuration != nil {
		params.Configuration = req.Configuration
	}
	// TODO: Add IsActive to the request model if updatable?

	dbKB, err := s.store.UpdateKnowledgeBase(ctx, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrKBNotFound
		}
		log.Printf("ERROR [KBService] UpdateKB: Store call failed for ID %s, OrgID %s: %v", id, orgID, err)
		return nil, fmt.Errorf("failed to update knowledge base: %w", err)
	}

	resp := mapDbKBToResponse(dbKB)
	log.Printf("[KBService] UpdateKnowledgeBase: Successfully updated KB ID %s for OrgID %s", resp.ID, orgID)
	return resp, nil
}

// DeleteKnowledgeBase deletes a KB by ID for the organization.
func (s *kbService) DeleteKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	err := s.store.DeleteKnowledgeBase(ctx, id, orgID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrKBNotFound
		}
		// Handle potential FK constraint error (e.g., if still mapped to chatbot)
		// if errors.Is(err, specificStoreFKError) { return ErrKBInUse }
		log.Printf("ERROR [KBService] DeleteKB: Store call failed for ID %s, OrgID %s: %v", id, orgID, err)
		return fmt.Errorf("failed to delete knowledge base: %w", err)
	}
	log.Printf("[KBService] DeleteKnowledgeBase: Successfully deleted KB ID %s for OrgID %s", id, orgID)
	return nil
}
