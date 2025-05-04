package postgres

import (
	db_models "buildmychat-backend/internal/models"
	"buildmychat-backend/internal/store"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Helper struct for JSONB storage of encrypted data
type encryptedDataJSON struct {
	Data string `json:"data"` // Base64 encoded encrypted bytes
}

// CreateIntegrationCredential inserts a new encrypted credential record.
func (s *PostgresStore) CreateIntegrationCredential(ctx context.Context, arg store.CreateIntegrationCredentialParams) (*db_models.IntegrationCredential, error) {
	log.Printf("[PostgresStore] CreateIntegrationCredential called for OrgID: %s, Name: %s, Service: %s", arg.OrganizationID, arg.CredentialName, arg.ServiceType)
	query := `
        INSERT INTO integration_credentials (id, organization_id, service_type, credential_name, encrypted_credentials, status)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, organization_id, service_type, credential_name, encrypted_credentials, status, created_at, updated_at`

	// Prepare JSONB data: base64 encode the raw encrypted bytes
	jsonData := encryptedDataJSON{Data: base64.StdEncoding.EncodeToString(arg.EncryptedCredentials)}
	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		log.Printf("ERROR [PostgresStore] CreateIntegrationCredential: Failed to marshal encrypted data to JSON for OrgID %s: %v", arg.OrganizationID, err)
		return nil, fmt.Errorf("failed to prepare encrypted credentials for storage: %w", err)
	}

	cred := &db_models.IntegrationCredential{}
	var storedJSONBytes []byte // Variable to scan the JSONB data from the DB

	err = s.db.QueryRow(ctx, query,
		arg.ID,
		arg.OrganizationID,
		arg.ServiceType,
		arg.CredentialName,
		jsonBytes, // Store the marshaled JSON containing base64 string
		arg.Status,
	).Scan(
		&cred.ID,
		&cred.OrganizationID,
		&cred.ServiceType,
		&cred.CredentialName,
		&storedJSONBytes, // Scan the stored JSONB bytes
		&cred.Status,
		&cred.CreatedAt,
		&cred.UpdatedAt,
	)

	if err != nil {
		log.Printf("ERROR [PostgresStore] CreateIntegrationCredential: Failed to execute/scan insert for OrgID %s: %v", arg.OrganizationID, err)
		return nil, fmt.Errorf("database error creating integration credential: %w", err)
	}

	// Decode the stored data back into raw bytes for the returned struct
	var retrievedData encryptedDataJSON
	if err := json.Unmarshal(storedJSONBytes, &retrievedData); err != nil {
		log.Printf("ERROR [PostgresStore] CreateIntegrationCredential: Failed to unmarshal stored JSONB for CredID %s: %v", cred.ID, err)
		return nil, fmt.Errorf("failed to process stored encrypted credentials: %w", err)
	}
	decodedBytes, err := base64.StdEncoding.DecodeString(retrievedData.Data)
	if err != nil {
		log.Printf("ERROR [PostgresStore] CreateIntegrationCredential: Failed to decode base64 data for CredID %s: %v", cred.ID, err)
		return nil, fmt.Errorf("failed to decode stored encrypted credentials: %w", err)
	}
	cred.EncryptedCredentials = decodedBytes // Store raw bytes in the struct

	log.Printf("[PostgresStore] CreateIntegrationCredential: Successfully inserted CredID %s for OrgID %s", cred.ID, cred.OrganizationID)
	return cred, nil
}

// GetIntegrationCredentialByID retrieves a credential ensuring it belongs to the org.
func (s *PostgresStore) GetIntegrationCredentialByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*db_models.IntegrationCredential, error) {
	log.Printf("[PostgresStore] GetIntegrationCredentialByID called for ID: %s, OrgID: %s", id, orgID)
	query := `
        SELECT id, organization_id, service_type, credential_name, encrypted_credentials, status, created_at, updated_at
        FROM integration_credentials
        WHERE id = $1 AND organization_id = $2`

	cred := &db_models.IntegrationCredential{}
	var storedJSONBytes []byte

	err := s.db.QueryRow(ctx, query, id, orgID).Scan(
		&cred.ID,
		&cred.OrganizationID,
		&cred.ServiceType,
		&cred.CredentialName,
		&storedJSONBytes,
		&cred.Status,
		&cred.CreatedAt,
		&cred.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[PostgresStore] GetIntegrationCredentialByID: Credential not found for ID %s, OrgID %s", id, orgID)
			return nil, store.ErrNotFound
		}
		log.Printf("ERROR [PostgresStore] GetIntegrationCredentialByID: Failed query/scan for ID %s, OrgID %s: %v", id, orgID, err)
		return nil, fmt.Errorf("database error fetching integration credential: %w", err)
	}

	// Decode the stored data
	var retrievedData encryptedDataJSON
	if err := json.Unmarshal(storedJSONBytes, &retrievedData); err != nil {
		log.Printf("ERROR [PostgresStore] GetIntegrationCredentialByID: Failed to unmarshal stored JSONB for CredID %s: %v", cred.ID, err)
		return nil, fmt.Errorf("failed to process stored encrypted credentials: %w", err)
	}
	decodedBytes, err := base64.StdEncoding.DecodeString(retrievedData.Data)
	if err != nil {
		log.Printf("ERROR [PostgresStore] GetIntegrationCredentialByID: Failed to decode base64 data for CredID %s: %v", cred.ID, err)
		return nil, fmt.Errorf("failed to decode stored encrypted credentials: %w", err)
	}
	cred.EncryptedCredentials = decodedBytes

	log.Printf("[PostgresStore] GetIntegrationCredentialByID: Found CredID %s for OrgID %s", cred.ID, cred.OrganizationID)
	return cred, nil
}

// ListIntegrationCredentialsByOrg lists credentials for an organization, optionally filtering by type.
func (s *PostgresStore) ListIntegrationCredentialsByOrg(ctx context.Context, orgID uuid.UUID, serviceType *string) ([]db_models.IntegrationCredential, error) {
	log.Printf("[PostgresStore] ListIntegrationCredentialsByOrg called for OrgID: %s, ServiceTypeFilter: %v", orgID, serviceType)
	baseQuery := `
        SELECT id, organization_id, service_type, credential_name, encrypted_credentials, status, created_at, updated_at
        FROM integration_credentials
        WHERE organization_id = $1`

	args := []interface{}{orgID}
	query := baseQuery
	if serviceType != nil && *serviceType != "" {
		query += " AND service_type = $2"
		args = append(args, *serviceType)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("ERROR [PostgresStore] ListIntegrationCredentialsByOrg: Failed query for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("database error listing integration credentials: %w", err)
	}
	defer rows.Close()

	credentials := []db_models.IntegrationCredential{}
	for rows.Next() {
		cred := db_models.IntegrationCredential{}
		var storedJSONBytes []byte
		if err := rows.Scan(
			&cred.ID,
			&cred.OrganizationID,
			&cred.ServiceType,
			&cred.CredentialName,
			&storedJSONBytes,
			&cred.Status,
			&cred.CreatedAt,
			&cred.UpdatedAt,
		); err != nil {
			log.Printf("ERROR [PostgresStore] ListIntegrationCredentialsByOrg: Failed scanning row for OrgID %s: %v", orgID, err)
			return nil, fmt.Errorf("database error scanning integration credential: %w", err)
		}

		// Decode the stored data
		var retrievedData encryptedDataJSON
		if err := json.Unmarshal(storedJSONBytes, &retrievedData); err != nil {
			log.Printf("ERROR [PostgresStore] ListIntegrationCredentialsByOrg: Failed to unmarshal stored JSONB for CredID %s: %v", cred.ID, err)
			// Skip this credential or return error? Returning error for safety.
			return nil, fmt.Errorf("failed to process stored encrypted credentials for %s: %w", cred.ID, err)
		}
		decodedBytes, err := base64.StdEncoding.DecodeString(retrievedData.Data)
		if err != nil {
			log.Printf("ERROR [PostgresStore] ListIntegrationCredentialsByOrg: Failed to decode base64 data for CredID %s: %v", cred.ID, err)
			return nil, fmt.Errorf("failed to decode stored encrypted credentials for %s: %w", cred.ID, err)
		}
		cred.EncryptedCredentials = decodedBytes
		credentials = append(credentials, cred)
	}

	if err = rows.Err(); err != nil {
		log.Printf("ERROR [PostgresStore] ListIntegrationCredentialsByOrg: Error after iterating rows for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("database error after listing integration credentials: %w", err)
	}

	log.Printf("[PostgresStore] ListIntegrationCredentialsByOrg: Found %d credentials for OrgID %s", len(credentials), orgID)
	return credentials, nil
}

// UpdateIntegrationCredentialStatus updates the status of a specific credential.
func (s *PostgresStore) UpdateIntegrationCredentialStatus(ctx context.Context, id uuid.UUID, orgID uuid.UUID, status string) error {
	log.Printf("[PostgresStore] UpdateIntegrationCredentialStatus called for ID: %s, OrgID: %s, NewStatus: %s", id, orgID, status)
	query := `
        UPDATE integration_credentials
        SET status = $1, updated_at = now()
        WHERE id = $2 AND organization_id = $3`

	cmdTag, err := s.db.Exec(ctx, query, status, id, orgID)
	if err != nil {
		log.Printf("ERROR [PostgresStore] UpdateIntegrationCredentialStatus: Failed exec for ID %s, OrgID %s: %v", id, orgID, err)
		return fmt.Errorf("database error updating credential status: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		log.Printf("[PostgresStore] UpdateIntegrationCredentialStatus: Credential not found or not owned by org for ID %s, OrgID %s", id, orgID)
		return store.ErrNotFound // Use ErrNotFound if no rows were updated
	}

	log.Printf("[PostgresStore] UpdateIntegrationCredentialStatus: Successfully updated status for CredID %s", id)
	return nil
}

// DeleteIntegrationCredential deletes a credential ensuring it belongs to the org.
func (s *PostgresStore) DeleteIntegrationCredential(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	log.Printf("[PostgresStore] DeleteIntegrationCredential called for ID: %s, OrgID: %s", id, orgID)
	query := `DELETE FROM integration_credentials WHERE id = $1 AND organization_id = $2`

	cmdTag, err := s.db.Exec(ctx, query, id, orgID)
	if err != nil {
		// Check for foreign key constraint errors if delete is restricted (e.g., from knowledge_bases)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			log.Printf("WARN [PostgresStore] DeleteIntegrationCredential: Foreign key violation for ID %s, OrgID %s: %v", id, orgID, err)
			return fmt.Errorf("cannot delete credential because it is still in use by a Knowledge Base or Interface")
		}
		log.Printf("ERROR [PostgresStore] DeleteIntegrationCredential: Failed exec for ID %s, OrgID %s: %v", id, orgID, err)
		return fmt.Errorf("database error deleting integration credential: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		log.Printf("[PostgresStore] DeleteIntegrationCredential: Credential not found or not owned by org for ID %s, OrgID %s", id, orgID)
		return store.ErrNotFound // Use ErrNotFound if no rows were deleted
	}

	log.Printf("[PostgresStore] DeleteIntegrationCredential: Successfully deleted CredID %s for OrgID %s", id, orgID)
	return nil
}
