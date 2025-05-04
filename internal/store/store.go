package store

import (
	db_models "buildmychat-backend/internal/models"
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a specific record is not found.
var ErrNotFound = errors.New("record not found")

// CreateIntegrationCredentialParams contains parameters for creating a credential.
// We pass encrypted bytes directly, assuming JSONB handling (base64 wrapping) happens in the implementation.
type CreateIntegrationCredentialParams struct {
	ID                   uuid.UUID
	OrganizationID       uuid.UUID
	ServiceType          string // Use string here, map from models.ServiceType in service
	CredentialName       string
	EncryptedCredentials []byte // Raw encrypted bytes
	Status               string
}

// CreateKnowledgeBaseParams contains parameters for creating a knowledge base.
type CreateKnowledgeBaseParams struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	CredentialID   uuid.UUID
	ServiceType    string // Expect models.ServiceTypeNotion
	Name           string
	Configuration  []byte // JSON marshaled bytes
	IsActive       bool
}

// UpdateKnowledgeBaseParams contains parameters for updating a knowledge base.
type UpdateKnowledgeBaseParams struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           *string // Pointer to allow optional update
	Configuration  []byte  // JSON marshaled bytes, optional update
	IsActive       *bool   // Pointer to allow optional update
}

// CreateInterfaceParams contains parameters for creating an interface.
type CreateInterfaceParams struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	CredentialID   uuid.UUID
	ServiceType    string // Expect models.ServiceTypeSlack
	Name           string
	Configuration  []byte // JSON marshaled bytes
	IsActive       bool
}

// UpdateInterfaceParams contains parameters for updating an interface.
type UpdateInterfaceParams struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           *string // Pointer to allow optional update
	Configuration  []byte  // JSON marshaled bytes, optional update
	IsActive       *bool   // Pointer to allow optional update
}

// Store defines the interface for database operations.
// This allows for mocking in tests and potential DB backend switching.
type Store interface {
	// User operations
	GetUserByEmail(ctx context.Context, email string) (*db_models.User, error)
	CreateUser(ctx context.Context, user *db_models.User) error
	// GetUserByID(ctx context.Context, id uuid.UUID) (*db.User, error)
	// UpdateUser(ctx context.Context, user *db.User) error
	// DeleteUser(ctx context.Context, id uuid.UUID) error

	// Organization operations
	CreateOrganization(ctx context.Context, org *db_models.Organization) error
	// GetOrganizationByID(ctx context.Context, id uuid.UUID) (*db.Organization, error)
	// UpdateOrganization(ctx context.Context, org *db.Organization) error

	// Integration Credentials operations
	CreateIntegrationCredential(ctx context.Context, arg CreateIntegrationCredentialParams) (*db_models.IntegrationCredential, error)
	GetIntegrationCredentialByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*db_models.IntegrationCredential, error)
	ListIntegrationCredentialsByOrg(ctx context.Context, orgID uuid.UUID, serviceType *string) ([]db_models.IntegrationCredential, error) // Optional filter by type
	UpdateIntegrationCredentialStatus(ctx context.Context, id uuid.UUID, orgID uuid.UUID, status string) error
	DeleteIntegrationCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error

	// Knowledge Base operations
	CreateKnowledgeBase(ctx context.Context, arg CreateKnowledgeBaseParams) (*db_models.KnowledgeBase, error)
	GetKnowledgeBaseByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*db_models.KnowledgeBase, error)
	ListKnowledgeBasesByOrg(ctx context.Context, orgID uuid.UUID) ([]db_models.KnowledgeBase, error)
	UpdateKnowledgeBase(ctx context.Context, arg UpdateKnowledgeBaseParams) (*db_models.KnowledgeBase, error) // For config/status updates
	DeleteKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error

	// Interface operations
	CreateInterface(ctx context.Context, arg CreateInterfaceParams) (*db_models.Interface, error)
	GetInterfaceByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*db_models.Interface, error)
	ListInterfacesByOrg(ctx context.Context, orgID uuid.UUID) ([]db_models.Interface, error)
	UpdateInterface(ctx context.Context, arg UpdateInterfaceParams) (*db_models.Interface, error) // For config/status updates
	DeleteInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error

	// Add other interfaces for Chatbots, Mappings, Chats, etc.
	// ...
}

// Implementations below were moved to internal/store/postgres/store.go
