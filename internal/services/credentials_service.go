package services

import (
	"buildmychat-backend/internal/integrations"
	api_models "buildmychat-backend/internal/models"
	db_models "buildmychat-backend/internal/models"
	integration_models "buildmychat-backend/internal/models/integrations"
	"buildmychat-backend/internal/store"
	"context"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
)

// Custom errors for Credentials service
var (
	ErrCredentialNotFound     = errors.New("credential not found")
	ErrCredentialValidation   = errors.New("credential validation failed")
	ErrCredentialEncryption   = errors.New("credential encryption failed")
	ErrCredentialDecryption   = errors.New("credential decryption failed")
	ErrCredentialInUse        = errors.New("credential is in use and cannot be deleted")
	ErrCredentialTestFailed   = errors.New("credential test failed")
	ErrUnsupportedServiceType = errors.New("unsupported service type for testing")
)

// CredentialsService defines the interface for credential operations.
// This is useful if other services need to interact with credentials.
type CredentialsService interface {
	CreateCredential(ctx context.Context, req api_models.CreateCredentialRequest, orgID uuid.UUID) (*api_models.CredentialResponse, error)
	GetCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*api_models.CredentialResponse, error)
	ListCredentials(ctx context.Context, orgID uuid.UUID, serviceType *string) ([]api_models.CredentialResponse, error)
	DeleteCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error
	TestCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*api_models.TestCredentialResponse, error)
}

type credentialsService struct {
	store    store.Store
	aead     cipher.AEAD
	registry *integrations.Registry
}

// NewCredentialsService creates a new CredentialsService.
func NewCredentialsService(s store.Store, aeadCipher cipher.AEAD, reg *integrations.Registry) CredentialsService {
	return &credentialsService{
		store:    s,
		aead:     aeadCipher,
		registry: reg,
	}
}

// --- Helper Function ---
func mapDbCredentialToResponse(dbCred *db_models.IntegrationCredential) *api_models.CredentialResponse {
	return &api_models.CredentialResponse{
		ID:             dbCred.ID,
		OrganizationID: dbCred.OrganizationID,
		ServiceType:    dbCred.ServiceType,
		CredentialName: dbCred.CredentialName,
		Status:         dbCred.Status,
		CreatedAt:      dbCred.CreatedAt,
		UpdatedAt:      dbCred.UpdatedAt,
	}
}

// CreateCredential validates, encrypts, and stores new integration credentials.
func (s *credentialsService) CreateCredential(ctx context.Context, req api_models.CreateCredentialRequest, orgID uuid.UUID) (*api_models.CredentialResponse, error) {
	// Basic validation
	if req.ServiceType == "" {
		return nil, fmt.Errorf("%w: service type cannot be empty", ErrCredentialValidation)
	}
	if len(req.Credentials) == 0 {
		return nil, fmt.Errorf("%w: credentials map cannot be empty", ErrCredentialValidation)
	}

	// --- Pre-Save Test and Name Fetch ---
	var finalCredentialName string
	if req.CredentialName != nil {
		finalCredentialName = *req.CredentialName
	}

	if req.ServiceType == api_models.ServiceTypeNotion {
		log.Printf("[CredService] CreateCredential: Notion type detected, performing pre-save test for OrgID %s", orgID)
		// Get Notion integration handler
		integration, err := s.registry.Get(string(api_models.ServiceTypeNotion))
		if err != nil {
			// Should not happen if registry is initialized correctly
			log.Printf("ERROR [CredService] CreateCredential: Failed to get Notion integration handler: %v", err)
			return nil, fmt.Errorf("internal configuration error for Notion service")
		}

		// Test connection using the RAW, unencrypted credentials from the request
		testResult, err := integration.TestConnection(ctx, req.Credentials)
		if err != nil {
			// System error during test
			log.Printf("ERROR [CredService] CreateCredential: Notion TestConnection system error for OrgID %s: %v", orgID, err)
			return nil, fmt.Errorf("failed to test Notion connection: %w", err)
		}

		if !testResult.Success {
			// Test failed (e.g., invalid key)
			log.Printf("WARN [CredService] CreateCredential: Notion pre-save test failed for OrgID %s: %s", orgID, testResult.Message)
			return nil, fmt.Errorf("%w: %s", ErrCredentialTestFailed, testResult.Message)
		}

		// Test succeeded, get the bot name from details
		if botName, ok := testResult.Details["bot_name"].(string); ok && botName != "" {
			finalCredentialName = botName
			log.Printf("[CredService] CreateCredential: Notion test successful. Using fetched bot name: '%s' for OrgID %s", finalCredentialName, orgID)
		} else {
			log.Printf("WARN [CredService] CreateCredential: Notion test successful but failed to extract bot name for OrgID %s. Using provided/default name: '%s'", orgID, finalCredentialName)
		}
	} else if req.ServiceType == api_models.ServiceTypeSlack { // Added block for Slack
		log.Printf("[CredService] CreateCredential: Slack type detected, performing pre-save test for OrgID %s", orgID)
		integration, err := s.registry.Get(string(api_models.ServiceTypeSlack))
		if err != nil {
			log.Printf("ERROR [CredService] CreateCredential: Failed to get Slack integration handler: %v", err)
			return nil, fmt.Errorf("internal configuration error for Slack service")
		}

		testResult, err := integration.TestConnection(ctx, req.Credentials)
		if err != nil {
			log.Printf("ERROR [CredService] CreateCredential: Slack TestConnection system error for OrgID %s: %v", orgID, err)
			return nil, fmt.Errorf("failed to test Slack connection: %w", err)
		}

		if !testResult.Success {
			log.Printf("WARN [CredService] CreateCredential: Slack pre-save test failed for OrgID %s: %s", orgID, testResult.Message)
			return nil, fmt.Errorf("%w: %s", ErrCredentialTestFailed, testResult.Message)
		}

		// Use fetched bot name if available
		if botName, ok := testResult.Details["bot_name"].(string); ok && botName != "" {
			finalCredentialName = botName
			log.Printf("[CredService] CreateCredential: Slack test successful. Using fetched bot name: '%s' for OrgID %s", finalCredentialName, orgID)
		} else {
			log.Printf("WARN [CredService] CreateCredential: Slack test successful but failed to extract bot name for OrgID %s. Using provided/default name: '%s'", orgID, finalCredentialName)
		}
	}
	// --- End Pre-Save Test ---

	// Marshal the raw credentials map to JSON bytes (using req.Credentials)
	plaintextBytes, err := json.Marshal(req.Credentials)
	if err != nil {
		log.Printf("ERROR [CredService] CreateCredential: Failed marshal credentials for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to process credentials data: %w", err)
	}

	// Encrypt the JSON bytes
	encryptedBytes, err := s.encrypt(plaintextBytes)
	if err != nil {
		log.Printf("ERROR [CredService] CreateCredential: Encryption failed for OrgID %s: %v", orgID, err)
		return nil, ErrCredentialEncryption
	}

	// Wrap encrypted bytes in JSON structure
	encryptedBase64 := base64.StdEncoding.EncodeToString(encryptedBytes)
	encryptedWrapper := struct {
		Encrypted string `json:"encrypted"`
	}{
		Encrypted: encryptedBase64,
	}
	wrappedJSONBytes, err := json.Marshal(encryptedWrapper)
	if err != nil {
		log.Printf("ERROR [CredService] CreateCredential: Failed marshal wrapper for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to prepare encrypted data for storage: %w", err)
	}

	// Prepare params for store
	params := store.CreateIntegrationCredentialParams{
		ID:                   uuid.New(),
		OrganizationID:       orgID,
		ServiceType:          string(req.ServiceType),
		CredentialName:       finalCredentialName, // Use the final name (fetched or provided or empty)
		EncryptedCredentials: wrappedJSONBytes,
		Status:               "ACTIVE",
	}

	// Call store to create the credential
	dbCred, err := s.store.CreateIntegrationCredential(ctx, params)
	if err != nil {
		log.Printf("ERROR [CredService] CreateCredential: Store call failed for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to save credential: %w", err)
	}

	// Map to response DTO
	resp := mapDbCredentialToResponse(dbCred)

	log.Printf("[CredService] CreateCredential: Successfully created CredID %s for OrgID %s with Name '%s'", resp.ID, orgID, resp.CredentialName)
	return resp, nil
}

// GetCredential retrieves a credential by ID for the specified organization.
func (s *credentialsService) GetCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*api_models.CredentialResponse, error) {
	dbCred, err := s.store.GetIntegrationCredentialByID(ctx, id, orgID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrCredentialNotFound
		}
		log.Printf("ERROR [CredService] GetCredential: Store call failed for ID %s, OrgID %s: %v", id, orgID, err)
		return nil, fmt.Errorf("failed to retrieve credential: %w", err)
	}

	// Map to response DTO
	resp := mapDbCredentialToResponse(dbCred)
	return resp, nil
}

// ListCredentials retrieves all credentials for the specified organization.
func (s *credentialsService) ListCredentials(ctx context.Context, orgID uuid.UUID, serviceType *string) ([]api_models.CredentialResponse, error) {
	dbCreds, err := s.store.ListIntegrationCredentialsByOrg(ctx, orgID, serviceType)
	if err != nil {
		log.Printf("ERROR [CredService] ListCredentials: Store call failed for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	// Map to response DTOs
	resp := make([]api_models.CredentialResponse, len(dbCreds))
	for i, dbCred := range dbCreds {
		// Create a new variable in the loop scope for the pointer
		cred := dbCred
		resp[i] = *mapDbCredentialToResponse(&cred)
	}

	return resp, nil
}

// DeleteCredential deletes a credential by ID for the specified organization.
func (s *credentialsService) DeleteCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	err := s.store.DeleteIntegrationCredential(ctx, id, orgID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrCredentialNotFound
		}
		// Check if the error message indicates it's in use (from FK constraint)
		if err.Error() == "cannot delete credential because it is still in use by a Knowledge Base or Interface" {
			return ErrCredentialInUse
		}
		log.Printf("ERROR [CredService] DeleteCredential: Store call failed for ID %s, OrgID %s: %v", id, orgID, err)
		return fmt.Errorf("failed to delete credential: %w", err)
	}
	log.Printf("[CredService] DeleteCredential: Successfully deleted CredID %s for OrgID %s", id, orgID)
	return nil
}

// --- Helper for Decryption ---
// decryptCredentials helper function
func (s *credentialsService) decryptCredentials(encryptedBase64 string) (integration_models.DecryptedCredentials, error) {
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 credentials: %w", err)
	}

	decryptedJSON, err := s.decrypt(encryptedBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	var decryptedCreds integration_models.DecryptedCredentials
	if err := json.Unmarshal(decryptedJSON, &decryptedCreds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal decrypted credentials JSON: %w", err)
	}
	return decryptedCreds, nil
}

// TestCredential attempts to verify the credential by connecting to the external service.
func (s *credentialsService) TestCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*api_models.TestCredentialResponse, error) {
	log.Printf("[CredentialsService] TestCredential - Starting test for CredID: %s, OrgID: %s", id, orgID)
	// 1. Get the credential from the store
	dbCred, err := s.store.GetIntegrationCredentialByID(ctx, id, orgID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			log.Printf("[CredentialsService] TestCredential - Credential not found for ID: %s", id)
			return nil, ErrCredentialNotFound
		}
		log.Printf("ERROR [CredentialsService] TestCredential - GetByID failed for ID %s, OrgID %s: %v", id, orgID, err)
		return nil, fmt.Errorf("failed to retrieve credential: %w", err)
	}
	log.Printf("[CredentialsService] TestCredential - Found credential, ServiceType: %s", dbCred.ServiceType)

	// 2. Get the integration handler from the registry
	integration, err := s.registry.Get(string(dbCred.ServiceType))
	if err != nil {
		// This should ideally not happen if service types are validated on creation
		log.Printf("ERROR [CredentialsService] TestCredential - Registry lookup failed for type %s, ID %s: %v", dbCred.ServiceType, id, err)
		return nil, fmt.Errorf("internal error: unsupported service type '%s'", dbCred.ServiceType)
	}
	log.Printf("[CredentialsService] TestCredential - Found integration handler for type: %s", dbCred.ServiceType)

	// 3. Decrypt the credentials
	// The stored value is JSON like `{"encrypted": "base64..."}`. We need the base64 part.
	var encryptedWrapper struct {
		Encrypted string `json:"encrypted"`
	}
	// Directly unmarshal the JSON bytes from the database field
	if err := json.Unmarshal(dbCred.EncryptedCredentials, &encryptedWrapper); err != nil {
		log.Printf("ERROR [CredentialsService] TestCredential - Failed unmarshalling wrapper for ID %s: %v", id, err)
		return nil, fmt.Errorf("internal error: failed to read stored credentials format")
	}
	log.Printf("[CredentialsService] TestCredential - Successfully unmarshalled encrypted wrapper for ID: %s", id)

	decryptedCredsMap, err := s.decryptCredentials(encryptedWrapper.Encrypted)
	if err != nil {
		log.Printf("ERROR [CredentialsService] TestCredential - Decryption failed for ID %s: %v", id, err)
		// Don't expose decryption error details usually, but maybe signal internal issue
		return &api_models.TestCredentialResponse{
			Success: false,
			Message: "Failed to decrypt credentials for testing.",
		}, nil // Return error as part of the response, not a 500
	}
	// --- IMPORTANT: Log only the KEYS, not the sensitive values! ---
	decryptedKeys := make([]string, 0, len(decryptedCredsMap))
	for k := range decryptedCredsMap {
		decryptedKeys = append(decryptedKeys, k)
	}
	log.Printf("[CredentialsService] TestCredential - Successfully decrypted. Found credential keys: %v for ID: %s", decryptedKeys, id)
	// --- End Security Note ---

	// 4. Call the integration's TestConnection method
	log.Printf("[CredentialsService] TestCredential - Calling integration.TestConnection for ID: %s", id)
	testResult, err := integration.TestConnection(ctx, decryptedCredsMap)
	if err != nil {
		// This indicates a system error during the test (e.g., network issue, unexpected panic)
		log.Printf("ERROR [CredentialsService] TestCredential - integration.TestConnection failed for ID %s: %v", id, err)
		return nil, fmt.Errorf("error occurred during connection test: %w", err)
	}
	log.Printf("[CredentialsService] TestCredential - integration.TestConnection completed for ID: %s. Success: %v, Message: '%s'", id, testResult.Success, testResult.Message)

	// 5. Map the result to the API response
	return &api_models.TestCredentialResponse{
		Success: testResult.Success,
		Message: testResult.Message,
	}, nil
}

// --- Encryption/Decryption --- (These should already exist)
func (s *credentialsService) encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	// FIXME: Implement proper nonce generation (e.g., crypto/rand)
	if len(nonce) == 0 {
		return nil, errors.New("failed to generate nonce")
	}
	return s.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (s *credentialsService) decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := s.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, encryptedMessage := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return s.aead.Open(nil, nonce, encryptedMessage, nil)
}
