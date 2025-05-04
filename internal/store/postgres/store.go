package postgres

import (
	db_models "buildmychat-backend/internal/models"
	"buildmychat-backend/internal/store"
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check to ensure PostgresStore implements store.Store
var _ store.Store = (*PostgresStore)(nil)

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(db *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{db: db}
}

// GetUserByEmail retrieves a user by their email address.
// Returns store.ErrNotFound if the user does not exist.
func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*db_models.User, error) {
	log.Printf("[PostgresStore] GetUserByEmail called for: %s", email)
	query := `
		SELECT id, organization_id, email, hashed_password, created_at, updated_at
		FROM users
		WHERE email = $1`

	user := &db_models.User{}
	err := s.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.HashedPassword,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[PostgresStore] GetUserByEmail: User not found for email %s", email)
			return nil, store.ErrNotFound // Return specific not found error
		}
		log.Printf("ERROR [PostgresStore] GetUserByEmail: Failed to query/scan user for email %s: %v", email, err)
		return nil, fmt.Errorf("database error fetching user by email: %w", err)
	}

	log.Printf("[PostgresStore] GetUserByEmail: Found user ID %s for email %s", user.ID, email)
	return user, nil
}

// CreateUser inserts a new user record into the database.
func (s *PostgresStore) CreateUser(ctx context.Context, user *db_models.User) error {
	log.Printf("[PostgresStore] CreateUser called for: %s (OrgID: %s, UserID: %s)", user.Email, user.OrganizationID, user.ID)
	query := `
		INSERT INTO users (id, organization_id, email, hashed_password)
		VALUES ($1, $2, $3, $4)`
	// created_at and updated_at should have database defaults (e.g., NOW())

	_, err := s.db.Exec(ctx, query,
		user.ID,
		user.OrganizationID,
		user.Email,
		user.HashedPassword,
	)

	if err != nil {
		// Check for specific constraint errors, e.g., duplicate email or ID if constraints exist
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// You can check pgErr.Code for specific PostgreSQL error codes
			// Example: "23505" is unique_violation
			log.Printf("ERROR [PostgresStore] CreateUser: PostgreSQL error executing insert for email %s: Code=%s, Message=%s, Detail=%s", user.Email, pgErr.Code, pgErr.Message, pgErr.Detail)
		} else {
			log.Printf("ERROR [PostgresStore] CreateUser: Failed to execute insert for email %s: %v", user.Email, err)
		}
		return fmt.Errorf("database error creating user: %w", err)
	}

	log.Printf("[PostgresStore] CreateUser: Successfully inserted user ID %s for email %s", user.ID, user.Email)
	return nil
}

// CreateOrganization inserts a new organization record into the database.
func (s *PostgresStore) CreateOrganization(ctx context.Context, org *db_models.Organization) error {
	log.Printf("[PostgresStore] CreateOrganization called for: %s (OrgID: %s)", org.Name, org.ID)
	query := `
		INSERT INTO organizations (id, name)
		VALUES ($1, $2)`
	// created_at and updated_at should have database defaults (e.g., NOW())

	_, err := s.db.Exec(ctx, query,
		org.ID,
		org.Name,
	)

	if err != nil {
		// Check for specific constraint errors
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			log.Printf("ERROR [PostgresStore] CreateOrganization: PostgreSQL error executing insert for org %s: Code=%s, Message=%s, Detail=%s", org.Name, pgErr.Code, pgErr.Message, pgErr.Detail)
		} else {
			log.Printf("ERROR [PostgresStore] CreateOrganization: Failed to execute insert for org %s: %v", org.Name, err)
		}
		return fmt.Errorf("database error creating organization: %w", err)
	}

	log.Printf("[PostgresStore] CreateOrganization: Successfully inserted organization ID %s for name %s", org.ID, org.Name)
	return nil
}

// Methods for other entities (Credentials, KB, Interface, etc.) are now in separate files
// (e.g., store_credentials.go, store_kb.go, store_interface.go)
