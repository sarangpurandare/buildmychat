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

// Custom errors for Interface service
var (
	ErrInterfaceNotFound           = errors.New("interface not found")
	ErrInterfaceValidation         = errors.New("interface validation failed")
	ErrInterfaceCredentialMismatch = errors.New("provided credential is not valid for this interface type (expected SLACK)")
)

// InterfaceService defines the interface for Interface operations.
type InterfaceService interface {
	CreateInterface(ctx context.Context, req api_models.CreateInterfaceRequest, orgID uuid.UUID) (*api_models.InterfaceResponse, error)
	GetInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*api_models.InterfaceResponse, error)
	ListInterfaces(ctx context.Context, orgID uuid.UUID) ([]api_models.InterfaceResponse, error)
	UpdateInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID, req api_models.CreateInterfaceRequest) (*api_models.InterfaceResponse, error) // Reuse Create req for update simplicity
	DeleteInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
}

type interfaceService struct {
	store store.Store
}

// NewInterfaceService creates a new InterfaceService.
func NewInterfaceService(s store.Store) InterfaceService {
	return &interfaceService{
		store: s,
	}
}

// --- Helper Function ---
func mapDbInterfaceToResponse(dbIntf *db_models.Interface) *api_models.InterfaceResponse {
	return &api_models.InterfaceResponse{
		ID:             dbIntf.ID,
		OrganizationID: dbIntf.OrganizationID,
		CredentialID:   dbIntf.CredentialID,
		ServiceType:    dbIntf.ServiceType,
		Name:           dbIntf.Name,
		Configuration:  dbIntf.Configuration,
		IsActive:       dbIntf.IsActive,
		CreatedAt:      dbIntf.CreatedAt,
		UpdatedAt:      dbIntf.UpdatedAt,
	}
}

// CreateInterface validates input, checks credential, and creates a new Interface.
func (s *interfaceService) CreateInterface(ctx context.Context, req api_models.CreateInterfaceRequest, orgID uuid.UUID) (*api_models.InterfaceResponse, error) {
	// Validate input
	if req.Name == "" {
		return nil, fmt.Errorf("%w: name cannot be empty", ErrInterfaceValidation)
	}
	if req.CredentialID == uuid.Nil {
		return nil, fmt.Errorf("%w: credential_id cannot be empty", ErrInterfaceValidation)
	}
	if req.Configuration != nil && !json.Valid(req.Configuration) {
		return nil, fmt.Errorf("%w: configuration is not valid JSON", ErrInterfaceValidation)
	}
	// TODO: Validate Slack configuration specifics (e.g., team_id format?)

	// Verify Credential exists, belongs to org, and is for SLACK
	cred, err := s.store.GetIntegrationCredentialByID(ctx, req.CredentialID, orgID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("%w: %v", ErrInterfaceValidation, err)
		}
		log.Printf("ERROR [InterfaceService] CreateInterface: Failed GetIntegrationCredentialByID for CredID %s, OrgID %s: %v", req.CredentialID, orgID, err)
		return nil, fmt.Errorf("failed to verify credential: %w", err)
	}
	if cred.ServiceType != api_models.ServiceTypeSlack {
		return nil, ErrInterfaceCredentialMismatch
	}

	params := store.CreateInterfaceParams{
		ID:             uuid.New(),
		OrganizationID: orgID,
		CredentialID:   req.CredentialID,
		ServiceType:    string(api_models.ServiceTypeSlack), // Hardcode for now
		Name:           req.Name,
		Configuration:  req.Configuration,
		IsActive:       false, // Default to inactive initially
	}

	dbIntf, err := s.store.CreateInterface(ctx, params)
	if err != nil {
		log.Printf("ERROR [InterfaceService] CreateInterface: Store call failed for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to save interface: %w", err)
	}

	resp := mapDbInterfaceToResponse(dbIntf)
	log.Printf("[InterfaceService] CreateInterface: Successfully created Interface ID %s for OrgID %s", resp.ID, orgID)
	return resp, nil
}

// GetInterface retrieves a specific Interface by ID for the organization.
func (s *interfaceService) GetInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*api_models.InterfaceResponse, error) {
	dbIntf, err := s.store.GetInterfaceByID(ctx, id, orgID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrInterfaceNotFound
		}
		log.Printf("ERROR [InterfaceService] GetInterface: Store call failed for ID %s, OrgID %s: %v", id, orgID, err)
		return nil, fmt.Errorf("failed to retrieve interface: %w", err)
	}
	return mapDbInterfaceToResponse(dbIntf), nil
}

// ListInterfaces retrieves all Interfaces for the organization.
func (s *interfaceService) ListInterfaces(ctx context.Context, orgID uuid.UUID) ([]api_models.InterfaceResponse, error) {
	dbIntfs, err := s.store.ListInterfacesByOrg(ctx, orgID)
	if err != nil {
		log.Printf("ERROR [InterfaceService] ListInterfaces: Store call failed for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	resp := make([]api_models.InterfaceResponse, len(dbIntfs))
	for i, dbIntf := range dbIntfs {
		intf := dbIntf // Avoid loop variable capture
		resp[i] = *mapDbInterfaceToResponse(&intf)
	}
	return resp, nil
}

// UpdateInterface updates an existing Interface.
func (s *interfaceService) UpdateInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID, req api_models.CreateInterfaceRequest) (*api_models.InterfaceResponse, error) {
	if req.Name != "" && len(req.Name) == 0 {
		return nil, fmt.Errorf("%w: name cannot be updated to empty", ErrInterfaceValidation)
	}
	if req.Configuration != nil && !json.Valid(req.Configuration) {
		return nil, fmt.Errorf("%w: configuration is not valid JSON", ErrInterfaceValidation)
	}

	if req.CredentialID != uuid.Nil {
		cred, err := s.store.GetIntegrationCredentialByID(ctx, req.CredentialID, orgID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, fmt.Errorf("%w: credential_id %s not found", ErrInterfaceValidation, req.CredentialID)
			}
			log.Printf("ERROR [InterfaceService] UpdateInterface: Failed GetIntegrationCredentialByID for CredID %s, OrgID %s: %v", req.CredentialID, orgID, err)
			return nil, fmt.Errorf("failed to verify credential: %w", err)
		}
		if cred.ServiceType != api_models.ServiceTypeSlack {
			return nil, ErrInterfaceCredentialMismatch
		}
		log.Printf("WARN [InterfaceService] UpdateInterface: Attempted to update CredentialID for Interface %s - This is not supported via this endpoint.", id)
	}

	params := store.UpdateInterfaceParams{
		ID:             id,
		OrganizationID: orgID,
	}
	if req.Name != "" {
		params.Name = &req.Name
	}
	if req.Configuration != nil {
		params.Configuration = req.Configuration
	}
	// TODO: Add IsActive to request?

	dbIntf, err := s.store.UpdateInterface(ctx, params)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrInterfaceNotFound
		}
		log.Printf("ERROR [InterfaceService] UpdateInterface: Store call failed for ID %s, OrgID %s: %v", id, orgID, err)
		return nil, fmt.Errorf("failed to update interface: %w", err)
	}

	resp := mapDbInterfaceToResponse(dbIntf)
	log.Printf("[InterfaceService] UpdateInterface: Successfully updated Interface ID %s for OrgID %s", resp.ID, orgID)
	return resp, nil
}

// DeleteInterface deletes an Interface by ID for the organization.
func (s *interfaceService) DeleteInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	err := s.store.DeleteInterface(ctx, id, orgID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrInterfaceNotFound
		}
		log.Printf("ERROR [InterfaceService] DeleteInterface: Store call failed for ID %s, OrgID %s: %v", id, orgID, err)
		return fmt.Errorf("failed to delete interface: %w", err)
	}
	log.Printf("[InterfaceService] DeleteInterface: Successfully deleted Interface ID %s for OrgID %s", id, orgID)
	return nil
}
