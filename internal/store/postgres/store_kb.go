package postgres

import (
	db_models "buildmychat-backend/internal/models"
	"buildmychat-backend/internal/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- Knowledge Base Methods ---

// CreateKnowledgeBase inserts a new knowledge base record.
func (s *PostgresStore) CreateKnowledgeBase(ctx context.Context, arg store.CreateKnowledgeBaseParams) (*db_models.KnowledgeBase, error) {
	log.Printf("[PostgresStore] CreateKnowledgeBase called for OrgID: %s, Name: %s", arg.OrganizationID, arg.Name)
	query := `
        INSERT INTO knowledge_bases (id, organization_id, credential_id, service_type, name, configuration, is_active)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id, organization_id, credential_id, service_type, name, configuration, is_active, created_at, updated_at`

	kb := &db_models.KnowledgeBase{}
	var configBytes []byte // Handle nullable JSONB
	if arg.Configuration != nil {
		configBytes = arg.Configuration
	} else {
		// If config is nil in params, store SQL NULL or default empty JSON '{}'
		configBytes = []byte("{}") // Storing empty JSON object by default
	}

	err := s.db.QueryRow(ctx, query,
		arg.ID,
		arg.OrganizationID,
		arg.CredentialID,
		arg.ServiceType,
		arg.Name,
		configBytes,
		arg.IsActive,
	).Scan(
		&kb.ID,
		&kb.OrganizationID,
		&kb.CredentialID,
		&kb.ServiceType,
		&kb.Name,
		&kb.Configuration, // Scan directly into json.RawMessage
		&kb.IsActive,
		&kb.CreatedAt,
		&kb.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23503" { // foreign_key_violation (credential_id)
				log.Printf("WARN [PostgresStore] CreateKnowledgeBase: Foreign key violation for OrgID %s, CredID %s: %v", arg.OrganizationID, arg.CredentialID, err)
				return nil, fmt.Errorf("invalid credential ID provided")
			}
		}
		log.Printf("ERROR [PostgresStore] CreateKnowledgeBase: Failed exec/scan for OrgID %s: %v", arg.OrganizationID, err)
		return nil, fmt.Errorf("database error creating knowledge base: %w", err)
	}

	log.Printf("[PostgresStore] CreateKnowledgeBase: Successfully inserted KB ID %s for OrgID %s", kb.ID, kb.OrganizationID)
	return kb, nil
}

// GetKnowledgeBaseByID retrieves a specific knowledge base by its ID and organization ID.
func (s *PostgresStore) GetKnowledgeBaseByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*db_models.KnowledgeBase, error) {
	log.Printf("[PostgresStore] GetKnowledgeBaseByID called for ID: %s, OrgID: %s", id, orgID)
	query := `
        SELECT id, organization_id, credential_id, service_type, name, configuration, is_active, created_at, updated_at
        FROM knowledge_bases
        WHERE id = $1 AND organization_id = $2`

	kb := &db_models.KnowledgeBase{}
	err := s.db.QueryRow(ctx, query, id, orgID).Scan(
		&kb.ID,
		&kb.OrganizationID,
		&kb.CredentialID,
		&kb.ServiceType,
		&kb.Name,
		&kb.Configuration,
		&kb.IsActive,
		&kb.CreatedAt,
		&kb.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[PostgresStore] GetKnowledgeBaseByID: Not found for ID %s, OrgID %s", id, orgID)
			return nil, store.ErrNotFound
		}
		log.Printf("ERROR [PostgresStore] GetKnowledgeBaseByID: Failed query/scan for ID %s, OrgID %s: %v", id, orgID, err)
		return nil, fmt.Errorf("database error fetching knowledge base: %w", err)
	}

	log.Printf("[PostgresStore] GetKnowledgeBaseByID: Found KB ID %s for OrgID %s", kb.ID, kb.OrganizationID)
	return kb, nil
}

// ListKnowledgeBasesByOrg retrieves all knowledge bases for a given organization.
func (s *PostgresStore) ListKnowledgeBasesByOrg(ctx context.Context, orgID uuid.UUID) ([]db_models.KnowledgeBase, error) {
	log.Printf("[PostgresStore] ListKnowledgeBasesByOrg called for OrgID: %s", orgID)
	query := `
        SELECT id, organization_id, credential_id, service_type, name, configuration, is_active, created_at, updated_at
        FROM knowledge_bases
        WHERE organization_id = $1
        ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, query, orgID)
	if err != nil {
		log.Printf("ERROR [PostgresStore] ListKnowledgeBasesByOrg: Failed query for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("database error listing knowledge bases: %w", err)
	}
	defer rows.Close()

	kbs := []db_models.KnowledgeBase{}
	for rows.Next() {
		kb := db_models.KnowledgeBase{}
		if err := rows.Scan(
			&kb.ID,
			&kb.OrganizationID,
			&kb.CredentialID,
			&kb.ServiceType,
			&kb.Name,
			&kb.Configuration,
			&kb.IsActive,
			&kb.CreatedAt,
			&kb.UpdatedAt,
		); err != nil {
			log.Printf("ERROR [PostgresStore] ListKnowledgeBasesByOrg: Failed scanning row for OrgID %s: %v", orgID, err)
			return nil, fmt.Errorf("database error scanning knowledge base: %w", err)
		}
		kbs = append(kbs, kb)
	}

	if err = rows.Err(); err != nil {
		log.Printf("ERROR [PostgresStore] ListKnowledgeBasesByOrg: Error after iterating rows for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("database error after listing knowledge bases: %w", err)
	}

	log.Printf("[PostgresStore] ListKnowledgeBasesByOrg: Found %d KBs for OrgID %s", len(kbs), orgID)
	return kbs, nil
}

// UpdateKnowledgeBase updates fields for a specific knowledge base.
// Uses COALESCE to only update fields provided in the args (non-nil pointers).
func (s *PostgresStore) UpdateKnowledgeBase(ctx context.Context, arg store.UpdateKnowledgeBaseParams) (*db_models.KnowledgeBase, error) {
	log.Printf("[PostgresStore] UpdateKnowledgeBase called for ID: %s, OrgID: %s", arg.ID, arg.OrganizationID)

	// Build query dynamically for partial updates
	setClauses := []string{"updated_at = now()"}
	args := []interface{}{arg.ID, arg.OrganizationID} // Start with mandatory WHERE args
	argCounter := 3                                   // Start $ counter after WHERE args

	if arg.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argCounter))
		args = append(args, *arg.Name)
		argCounter++
	}
	if arg.Configuration != nil {
		// Ensure valid JSON before attempting update
		if !json.Valid(arg.Configuration) {
			return nil, errors.New("invalid JSON format in configuration")
		}
		setClauses = append(setClauses, fmt.Sprintf("configuration = $%d", argCounter))
		args = append(args, arg.Configuration)
		argCounter++
	}
	if arg.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", argCounter))
		args = append(args, *arg.IsActive)
		argCounter++
	}

	if len(setClauses) == 1 { // Only updated_at = now()
		log.Printf("[PostgresStore] UpdateKnowledgeBase: No fields provided to update for ID %s", arg.ID)
		// Optionally return current state or specific error
		return s.GetKnowledgeBaseByID(ctx, arg.ID, arg.OrganizationID)
	}

	query := fmt.Sprintf(`
        UPDATE knowledge_bases
        SET %s
        WHERE id = $1 AND organization_id = $2
        RETURNING id, organization_id, credential_id, service_type, name, configuration, is_active, created_at, updated_at`,
		strings.Join(setClauses, ", "),
	)

	kb := &db_models.KnowledgeBase{}
	err := s.db.QueryRow(ctx, query, args...).Scan(
		&kb.ID,
		&kb.OrganizationID,
		&kb.CredentialID,
		&kb.ServiceType,
		&kb.Name,
		&kb.Configuration,
		&kb.IsActive,
		&kb.CreatedAt,
		&kb.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[PostgresStore] UpdateKnowledgeBase: Not found for ID %s, OrgID %s", arg.ID, arg.OrganizationID)
			return nil, store.ErrNotFound
		}
		// Handle potential unique constraint violation on name if updated
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			log.Printf("WARN [PostgresStore] UpdateKnowledgeBase: Unique constraint violation for OrgID %s, ID %s: %v", arg.OrganizationID, arg.ID, err)
			return nil, fmt.Errorf("knowledge base name conflicts with an existing one in this organization")
		}
		log.Printf("ERROR [PostgresStore] UpdateKnowledgeBase: Failed query/scan for ID %s, OrgID %s: %v", arg.ID, arg.OrganizationID, err)
		return nil, fmt.Errorf("database error updating knowledge base: %w", err)
	}

	log.Printf("[PostgresStore] UpdateKnowledgeBase: Successfully updated KB ID %s for OrgID %s", kb.ID, kb.OrganizationID)
	return kb, nil
}

// DeleteKnowledgeBase deletes a specific knowledge base by ID and organization ID.
func (s *PostgresStore) DeleteKnowledgeBase(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	log.Printf("[PostgresStore] DeleteKnowledgeBase called for ID: %s, OrgID: %s", id, orgID)
	query := `DELETE FROM knowledge_bases WHERE id = $1 AND organization_id = $2`

	cmdTag, err := s.db.Exec(ctx, query, id, orgID)
	if err != nil {
		// Check for foreign key errors if KBs are referenced elsewhere (e.g., mappings)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			log.Printf("WARN [PostgresStore] DeleteKnowledgeBase: Foreign key violation for ID %s, OrgID %s: %v", id, orgID, err)
			return fmt.Errorf("cannot delete knowledge base because it is still in use")
		}
		log.Printf("ERROR [PostgresStore] DeleteKnowledgeBase: Failed exec for ID %s, OrgID %s: %v", id, orgID, err)
		return fmt.Errorf("database error deleting knowledge base: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		log.Printf("[PostgresStore] DeleteKnowledgeBase: Not found for ID %s, OrgID %s", id, orgID)
		return store.ErrNotFound
	}

	log.Printf("[PostgresStore] DeleteKnowledgeBase: Successfully deleted KB ID %s for OrgID %s", id, orgID)
	return nil
}
