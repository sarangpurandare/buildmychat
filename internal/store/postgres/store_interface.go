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

// --- Interface Methods ---

// CreateInterface inserts a new interface record.
func (s *PostgresStore) CreateInterface(ctx context.Context, arg store.CreateInterfaceParams) (*db_models.Interface, error) {
	log.Printf("[PostgresStore] CreateInterface called for OrgID: %s, Name: %s", arg.OrganizationID, arg.Name)
	query := `
        INSERT INTO interfaces (id, organization_id, credential_id, service_type, name, configuration, is_active)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id, organization_id, credential_id, service_type, name, configuration, is_active, created_at, updated_at`

	intf := &db_models.Interface{}
	var configBytes []byte
	if arg.Configuration != nil {
		configBytes = arg.Configuration
	} else {
		configBytes = []byte("{}") // Default empty JSON
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
		&intf.ID,
		&intf.OrganizationID,
		&intf.CredentialID,
		&intf.ServiceType,
		&intf.Name,
		&intf.Configuration,
		&intf.IsActive,
		&intf.CreatedAt,
		&intf.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23503" { // foreign_key_violation (credential_id)
				log.Printf("WARN [PostgresStore] CreateInterface: Foreign key violation for OrgID %s, CredID %s: %v", arg.OrganizationID, arg.CredentialID, err)
				return nil, fmt.Errorf("invalid credential ID provided")
			}
		}
		log.Printf("ERROR [PostgresStore] CreateInterface: Failed exec/scan for OrgID %s: %v", arg.OrganizationID, err)
		return nil, fmt.Errorf("database error creating interface: %w", err)
	}

	log.Printf("[PostgresStore] CreateInterface: Successfully inserted Interface ID %s for OrgID %s", intf.ID, intf.OrganizationID)
	return intf, nil
}

// GetInterfaceByID retrieves a specific interface by its ID and organization ID.
func (s *PostgresStore) GetInterfaceByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*db_models.Interface, error) {
	log.Printf("[PostgresStore] GetInterfaceByID called for ID: %s, OrgID: %s", id, orgID)
	query := `
        SELECT id, organization_id, credential_id, service_type, name, configuration, is_active, created_at, updated_at
        FROM interfaces
        WHERE id = $1 AND organization_id = $2`

	intf := &db_models.Interface{}
	err := s.db.QueryRow(ctx, query, id, orgID).Scan(
		&intf.ID,
		&intf.OrganizationID,
		&intf.CredentialID,
		&intf.ServiceType,
		&intf.Name,
		&intf.Configuration,
		&intf.IsActive,
		&intf.CreatedAt,
		&intf.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[PostgresStore] GetInterfaceByID: Not found for ID %s, OrgID %s", id, orgID)
			return nil, store.ErrNotFound
		}
		log.Printf("ERROR [PostgresStore] GetInterfaceByID: Failed query/scan for ID %s, OrgID %s: %v", id, orgID, err)
		return nil, fmt.Errorf("database error fetching interface: %w", err)
	}

	log.Printf("[PostgresStore] GetInterfaceByID: Found Interface ID %s for OrgID %s", intf.ID, intf.OrganizationID)
	return intf, nil
}

// ListInterfacesByOrg retrieves all interfaces for a given organization.
func (s *PostgresStore) ListInterfacesByOrg(ctx context.Context, orgID uuid.UUID) ([]db_models.Interface, error) {
	log.Printf("[PostgresStore] ListInterfacesByOrg called for OrgID: %s", orgID)
	query := `
        SELECT id, organization_id, credential_id, service_type, name, configuration, is_active, created_at, updated_at
        FROM interfaces
        WHERE organization_id = $1
        ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, query, orgID)
	if err != nil {
		log.Printf("ERROR [PostgresStore] ListInterfacesByOrg: Failed query for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("database error listing interfaces: %w", err)
	}
	defer rows.Close()

	interfaces := []db_models.Interface{}
	for rows.Next() {
		intf := db_models.Interface{}
		if err := rows.Scan(
			&intf.ID,
			&intf.OrganizationID,
			&intf.CredentialID,
			&intf.ServiceType,
			&intf.Name,
			&intf.Configuration,
			&intf.IsActive,
			&intf.CreatedAt,
			&intf.UpdatedAt,
		); err != nil {
			log.Printf("ERROR [PostgresStore] ListInterfacesByOrg: Failed scanning row for OrgID %s: %v", orgID, err)
			return nil, fmt.Errorf("database error scanning interface: %w", err)
		}
		interfaces = append(interfaces, intf)
	}

	if err = rows.Err(); err != nil {
		log.Printf("ERROR [PostgresStore] ListInterfacesByOrg: Error after iterating rows for OrgID %s: %v", orgID, err)
		return nil, fmt.Errorf("database error after listing interfaces: %w", err)
	}

	log.Printf("[PostgresStore] ListInterfacesByOrg: Found %d Interfaces for OrgID %s", len(interfaces), orgID)
	return interfaces, nil
}

// UpdateInterface updates fields for a specific interface.
func (s *PostgresStore) UpdateInterface(ctx context.Context, arg store.UpdateInterfaceParams) (*db_models.Interface, error) {
	log.Printf("[PostgresStore] UpdateInterface called for ID: %s, OrgID: %s", arg.ID, arg.OrganizationID)

	setClauses := []string{"updated_at = now()"}
	args := []interface{}{arg.ID, arg.OrganizationID}
	argCounter := 3

	if arg.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argCounter))
		args = append(args, *arg.Name)
		argCounter++
	}
	if arg.Configuration != nil {
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

	if len(setClauses) == 1 { // Only updated_at
		log.Printf("[PostgresStore] UpdateInterface: No fields provided to update for ID %s", arg.ID)
		return s.GetInterfaceByID(ctx, arg.ID, arg.OrganizationID)
	}

	query := fmt.Sprintf(`
        UPDATE interfaces
        SET %s
        WHERE id = $1 AND organization_id = $2
        RETURNING id, organization_id, credential_id, service_type, name, configuration, is_active, created_at, updated_at`,
		strings.Join(setClauses, ", "),
	)

	intf := &db_models.Interface{}
	err := s.db.QueryRow(ctx, query, args...).Scan(
		&intf.ID,
		&intf.OrganizationID,
		&intf.CredentialID,
		&intf.ServiceType,
		&intf.Name,
		&intf.Configuration,
		&intf.IsActive,
		&intf.CreatedAt,
		&intf.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[PostgresStore] UpdateInterface: Not found for ID %s, OrgID %s", arg.ID, arg.OrganizationID)
			return nil, store.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			log.Printf("WARN [PostgresStore] UpdateInterface: Unique constraint violation for OrgID %s, ID %s: %v", arg.OrganizationID, arg.ID, err)
			return nil, fmt.Errorf("interface name conflicts with an existing one in this organization")
		}
		log.Printf("ERROR [PostgresStore] UpdateInterface: Failed query/scan for ID %s, OrgID %s: %v", arg.ID, arg.OrganizationID, err)
		return nil, fmt.Errorf("database error updating interface: %w", err)
	}

	log.Printf("[PostgresStore] UpdateInterface: Successfully updated Interface ID %s for OrgID %s", intf.ID, intf.OrganizationID)
	return intf, nil
}

// DeleteInterface deletes a specific interface by ID and organization ID.
func (s *PostgresStore) DeleteInterface(ctx context.Context, id uuid.UUID, orgID uuid.UUID) error {
	log.Printf("[PostgresStore] DeleteInterface called for ID: %s, OrgID: %s", id, orgID)
	query := `DELETE FROM interfaces WHERE id = $1 AND organization_id = $2`

	cmdTag, err := s.db.Exec(ctx, query, id, orgID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // Foreign key violation
			log.Printf("WARN [PostgresStore] DeleteInterface: Foreign key violation for ID %s, OrgID %s: %v", id, orgID, err)
			return fmt.Errorf("cannot delete interface because it is still in use")
		}
		log.Printf("ERROR [PostgresStore] DeleteInterface: Failed exec for ID %s, OrgID %s: %v", id, orgID, err)
		return fmt.Errorf("database error deleting interface: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		log.Printf("[PostgresStore] DeleteInterface: Not found for ID %s, OrgID %s", id, orgID)
		return store.ErrNotFound
	}

	log.Printf("[PostgresStore] DeleteInterface: Successfully deleted Interface ID %s for OrgID %s", id, orgID)
	return nil
}
