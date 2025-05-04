package postgres

import (
	"buildmychat-backend/internal/models"
	db_models "buildmychat-backend/internal/models"
	"buildmychat-backend/internal/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
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

// --- Chatbot Methods ---

const createChatbot = `-- name: CreateChatbot :one
INSERT INTO chatbots (
    organization_id, name, system_prompt, llm_model, configuration
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, organization_id, name, system_prompt, is_active, chat_count, llm_model, configuration, created_at, updated_at;
`

func (s *PostgresStore) CreateChatbot(ctx context.Context, arg store.CreateChatbotParams) (models.Chatbot, error) {
	row := s.db.QueryRow(ctx, createChatbot,
		arg.OrganizationID,
		arg.Name, // pgx handles *string to NULL automatically
		arg.SystemPrompt,
		arg.LLMModel,
		arg.Configuration, // pgx handles *json.RawMessage to NULL
	)
	var i models.Chatbot
	err := row.Scan(
		&i.ID,
		&i.OrganizationID,
		&i.Name,
		&i.SystemPrompt,
		&i.IsActive,
		&i.ChatCount,
		&i.LLMModel,
		&i.Configuration,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	// Handle potential default name scenario if needed, though DB handles it
	// if arg.Name == nil { // Example: Set a default name if nil was passed
	// 	 // update query here if needed, though creating with NULL is usually fine
	// }
	return i, err // pgx automatically maps pgx.ErrNoRows to store.ErrNotFound if Scan fails
}

const getChatbotByID = `-- name: GetChatbotByID :one
SELECT id, organization_id, name, system_prompt, is_active, chat_count, llm_model, configuration, created_at, updated_at
FROM chatbots
WHERE id = $1 AND organization_id = $2;
`

func (s *PostgresStore) GetChatbotByID(ctx context.Context, id uuid.UUID, organizationID uuid.UUID) (models.Chatbot, error) {
	row := s.db.QueryRow(ctx, getChatbotByID, id, organizationID)
	var i models.Chatbot
	err := row.Scan(
		&i.ID,
		&i.OrganizationID,
		&i.Name,
		&i.SystemPrompt,
		&i.IsActive,
		&i.ChatCount,
		&i.LLMModel,
		&i.Configuration,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Chatbot{}, store.ErrNotFound
		}
		return models.Chatbot{}, fmt.Errorf("error scanning chatbot: %w", err)
	}
	return i, nil
}

const listChatbots = `-- name: ListChatbots :many
SELECT id, organization_id, name, system_prompt, is_active, chat_count, llm_model, configuration, created_at, updated_at
FROM chatbots
WHERE organization_id = $1
ORDER BY created_at DESC;
`

func (s *PostgresStore) ListChatbots(ctx context.Context, organizationID uuid.UUID) ([]models.Chatbot, error) {
	rows, err := s.db.Query(ctx, listChatbots, organizationID)
	if err != nil {
		return nil, fmt.Errorf("error querying chatbots: %w", err)
	}
	defer rows.Close()

	var items []models.Chatbot
	for rows.Next() {
		var i models.Chatbot
		if err := rows.Scan(
			&i.ID,
			&i.OrganizationID,
			&i.Name,
			&i.SystemPrompt,
			&i.IsActive,
			&i.ChatCount,
			&i.LLMModel,
			&i.Configuration,
			&i.CreatedAt,
			&i.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning chatbot row: %w", err)
		}
		items = append(items, i)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chatbot rows: %w", err)
	}

	return items, nil
}

// UpdateChatbot builds the query dynamically based on which fields are provided.
func (s *PostgresStore) UpdateChatbot(ctx context.Context, arg store.UpdateChatbotParams) (models.Chatbot, error) {
	setClauses := []string{}
	args := []interface{}{}
	argID := 1

	if arg.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argID))
		args = append(args, *arg.Name)
		argID++
	}
	if arg.SystemPrompt != nil {
		setClauses = append(setClauses, fmt.Sprintf("system_prompt = $%d", argID))
		args = append(args, *arg.SystemPrompt)
		argID++
	}
	if arg.LLMModel != nil {
		setClauses = append(setClauses, fmt.Sprintf("llm_model = $%d", argID))
		args = append(args, *arg.LLMModel)
		argID++
	}
	if arg.Configuration != nil {
		setClauses = append(setClauses, fmt.Sprintf("configuration = $%d", argID))
		args = append(args, *arg.Configuration) // Pass the raw message directly
		argID++
	}

	if len(setClauses) == 0 {
		// No fields to update, maybe return the existing record or an error
		return s.GetChatbotByID(ctx, arg.ID, arg.OrganizationID)
	}

	// Always update the updated_at timestamp
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argID))
	args = append(args, time.Now())
	argID++

	// Add WHERE clause parameters
	args = append(args, arg.ID)
	args = append(args, arg.OrganizationID)

	query := fmt.Sprintf(`-- name: UpdateChatbot :one
		UPDATE chatbots
		SET %s
		WHERE id = $%d AND organization_id = $%d
		RETURNING id, organization_id, name, system_prompt, is_active, chat_count, llm_model, configuration, created_at, updated_at;`,
		strings.Join(setClauses, ", "),
		argID,   // ID placeholder index
		argID+1, // OrganizationID placeholder index
	)

	row := s.db.QueryRow(ctx, query, args...)
	var i models.Chatbot
	err := row.Scan(
		&i.ID,
		&i.OrganizationID,
		&i.Name,
		&i.SystemPrompt,
		&i.IsActive,
		&i.ChatCount,
		&i.LLMModel,
		&i.Configuration,
		&i.CreatedAt,
		&i.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// This could mean the chatbot didn't exist or org ID didn't match
			return models.Chatbot{}, store.ErrNotFound
		}
		return models.Chatbot{}, fmt.Errorf("error scanning updated chatbot: %w", err)
	}
	return i, nil
}

const updateChatbotStatus = `-- name: UpdateChatbotStatus :exec
UPDATE chatbots
SET is_active = $1, updated_at = NOW()
WHERE id = $2 AND organization_id = $3;
`

func (s *PostgresStore) UpdateChatbotStatus(ctx context.Context, id uuid.UUID, organizationID uuid.UUID, isActive bool) error {
	tag, err := s.db.Exec(ctx, updateChatbotStatus, isActive, id, organizationID)
	if err != nil {
		return fmt.Errorf("error executing update chatbot status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Could be due to wrong ID or OrgID not matching
		return store.ErrNotFound
	}
	return nil
}

const deleteChatbot = `-- name: DeleteChatbot :exec
DELETE FROM chatbots
WHERE id = $1 AND organization_id = $2;
`

func (s *PostgresStore) DeleteChatbot(ctx context.Context, id uuid.UUID, organizationID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, deleteChatbot, id, organizationID)
	if err != nil {
		return fmt.Errorf("error executing delete chatbot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Could be due to wrong ID or OrgID not matching
		return store.ErrNotFound
	}
	return nil
}

// --- Chatbot Mapping Methods ---

const addKnowledgeBaseMapping = `-- name: AddKnowledgeBaseMapping :exec
INSERT INTO chatbot_kb_mappings (
    chatbot_id, kb_id
) VALUES (
    $1, $2
)
ON CONFLICT (chatbot_id, kb_id) DO NOTHING;
`

func (s *PostgresStore) AddKnowledgeBaseMapping(ctx context.Context, chatbotID, kbID, orgID uuid.UUID) error {
	// First verify both IDs belong to the organization
	_, err := s.GetChatbotByID(ctx, chatbotID, orgID)
	if err != nil {
		return fmt.Errorf("failed to verify chatbot: %w", err)
	}

	_, err = s.GetKnowledgeBaseByID(ctx, kbID, orgID)
	if err != nil {
		return fmt.Errorf("failed to verify knowledge base: %w", err)
	}

	// Both exist and belong to the organization, proceed with mapping
	_, err = s.db.Exec(ctx, addKnowledgeBaseMapping, chatbotID, kbID)
	if err != nil {
		return fmt.Errorf("failed to add knowledge base mapping: %w", err)
	}

	return nil
}

const removeKnowledgeBaseMapping = `-- name: RemoveKnowledgeBaseMapping :exec
DELETE FROM chatbot_kb_mappings
WHERE chatbot_id = $1 AND kb_id = $2;
`

func (s *PostgresStore) RemoveKnowledgeBaseMapping(ctx context.Context, chatbotID, kbID, orgID uuid.UUID) error {
	// Verify chatbot belongs to organization
	_, err := s.GetChatbotByID(ctx, chatbotID, orgID)
	if err != nil {
		return fmt.Errorf("failed to verify chatbot: %w", err)
	}

	tag, err := s.db.Exec(ctx, removeKnowledgeBaseMapping, chatbotID, kbID)
	if err != nil {
		return fmt.Errorf("failed to remove knowledge base mapping: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}

	return nil
}

const addInterfaceMapping = `-- name: AddInterfaceMapping :exec
INSERT INTO chatbot_interface_mappings (
    chatbot_id, interface_id
) VALUES (
    $1, $2
)
ON CONFLICT (chatbot_id, interface_id) DO NOTHING;
`

func (s *PostgresStore) AddInterfaceMapping(ctx context.Context, chatbotID, interfaceID, orgID uuid.UUID) error {
	// First verify both IDs belong to the organization
	_, err := s.GetChatbotByID(ctx, chatbotID, orgID)
	if err != nil {
		return fmt.Errorf("failed to verify chatbot: %w", err)
	}

	_, err = s.GetInterfaceByID(ctx, interfaceID, orgID)
	if err != nil {
		return fmt.Errorf("failed to verify interface: %w", err)
	}

	// Both exist and belong to the organization, proceed with mapping
	_, err = s.db.Exec(ctx, addInterfaceMapping, chatbotID, interfaceID)
	if err != nil {
		return fmt.Errorf("failed to add interface mapping: %w", err)
	}

	return nil
}

const removeInterfaceMapping = `-- name: RemoveInterfaceMapping :exec
DELETE FROM chatbot_interface_mappings
WHERE chatbot_id = $1 AND interface_id = $2;
`

func (s *PostgresStore) RemoveInterfaceMapping(ctx context.Context, chatbotID, interfaceID, orgID uuid.UUID) error {
	// Verify chatbot belongs to organization
	_, err := s.GetChatbotByID(ctx, chatbotID, orgID)
	if err != nil {
		return fmt.Errorf("failed to verify chatbot: %w", err)
	}

	tag, err := s.db.Exec(ctx, removeInterfaceMapping, chatbotID, interfaceID)
	if err != nil {
		return fmt.Errorf("failed to remove interface mapping: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}

	return nil
}

func (s *PostgresStore) GetChatbotMappings(ctx context.Context, chatbotID, orgID uuid.UUID) (*models.ChatbotMappingsResponse, error) {
	// First verify the chatbot exists and belongs to the organization
	_, err := s.GetChatbotByID(ctx, chatbotID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify chatbot: %w", err)
	}

	// Fetch knowledge base mappings
	const getKBMappings = `
		SELECT kb.id, kb.organization_id, kb.credential_id, kb.service_type, kb.name, kb.configuration, kb.is_active, kb.created_at, kb.updated_at
		FROM knowledge_bases kb
		JOIN chatbot_kb_mappings map ON kb.id = map.kb_id
		WHERE map.chatbot_id = $1 AND kb.organization_id = $2
	`

	kbRows, err := s.db.Query(ctx, getKBMappings, chatbotID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch knowledge base mappings: %w", err)
	}
	defer kbRows.Close()

	var kbs []models.KnowledgeBase
	for kbRows.Next() {
		var kb models.KnowledgeBase
		if err := kbRows.Scan(
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
			return nil, fmt.Errorf("failed to scan knowledge base: %w", err)
		}
		kbs = append(kbs, kb)
	}
	if err = kbRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over knowledge base rows: %w", err)
	}

	// Fetch interface mappings
	const getInterfaceMappings = `
		SELECT i.id, i.organization_id, i.credential_id, i.service_type, i.name, i.configuration, i.is_active, i.created_at, i.updated_at
		FROM interfaces i
		JOIN chatbot_interface_mappings map ON i.id = map.interface_id
		WHERE map.chatbot_id = $1 AND i.organization_id = $2
	`

	ifaceRows, err := s.db.Query(ctx, getInterfaceMappings, chatbotID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch interface mappings: %w", err)
	}
	defer ifaceRows.Close()

	var ifaces []models.Interface
	for ifaceRows.Next() {
		var iface models.Interface
		if err := ifaceRows.Scan(
			&iface.ID,
			&iface.OrganizationID,
			&iface.CredentialID,
			&iface.ServiceType,
			&iface.Name,
			&iface.Configuration,
			&iface.IsActive,
			&iface.CreatedAt,
			&iface.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan interface: %w", err)
		}
		ifaces = append(ifaces, iface)
	}
	if err = ifaceRows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over interface rows: %w", err)
	}

	// Convert to response DTOs
	result := &models.ChatbotMappingsResponse{}

	if len(kbs) > 0 {
		result.KnowledgeBases = make([]models.KnowledgeBaseResponse, len(kbs))
		for i, kb := range kbs {
			result.KnowledgeBases[i] = models.KnowledgeBaseResponse{
				ID:             kb.ID,
				OrganizationID: kb.OrganizationID,
				CredentialID:   kb.CredentialID,
				ServiceType:    kb.ServiceType,
				Name:           kb.Name,
				Configuration:  kb.Configuration,
				IsActive:       kb.IsActive,
				CreatedAt:      kb.CreatedAt,
				UpdatedAt:      kb.UpdatedAt,
			}
		}
	}

	if len(ifaces) > 0 {
		result.Interfaces = make([]models.InterfaceResponse, len(ifaces))
		for i, iface := range ifaces {
			result.Interfaces[i] = models.InterfaceResponse{
				ID:             iface.ID,
				OrganizationID: iface.OrganizationID,
				CredentialID:   iface.CredentialID,
				ServiceType:    iface.ServiceType,
				Name:           iface.Name,
				Configuration:  iface.Configuration,
				IsActive:       iface.IsActive,
				CreatedAt:      iface.CreatedAt,
				UpdatedAt:      iface.UpdatedAt,
			}
		}
	}

	return result, nil
}

// --- Chat Methods ---

const createChat = `-- name: CreateChat :one
INSERT INTO chats (
    id, organization_id, chatbot_id, interface_id, external_chat_id, chat_data, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING id, chatbot_id, organization_id, interface_id, external_chat_id, chat_data, feedback, status, created_at, updated_at;
`

func (s *PostgresStore) CreateChat(ctx context.Context, arg store.CreateChatParams) (*models.Chat, error) {
	// Generate UUID if not provided
	id := arg.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	// Generate external chat ID if not provided
	externalChatID := arg.ExternalChatID
	if externalChatID == "" {
		externalChatID = uuid.New().String()
	}

	// Verify the chatbot exists and belongs to the organization
	_, err := s.GetChatbotByID(ctx, arg.ChatbotID, arg.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify chatbot: %w", err)
	}

	// Verify the interface exists and belongs to the organization
	_, err = s.GetInterfaceByID(ctx, arg.InterfaceID, arg.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify interface: %w", err)
	}

	// Use the provided chat data
	var chatData []byte
	if arg.ChatData != nil {
		chatData = arg.ChatData
	} else {
		// Default to empty array if no chat data provided
		chatData = []byte("[]")
	}

	// Default status is ACTIVE
	status := "ACTIVE"

	row := s.db.QueryRow(ctx, createChat,
		id,
		arg.OrganizationID,
		arg.ChatbotID,
		arg.InterfaceID,
		externalChatID,
		chatData,
		status,
	)

	var chat models.Chat
	err = row.Scan(
		&chat.ID,
		&chat.ChatbotID,
		&chat.OrganizationID,
		&chat.InterfaceID,
		&chat.ExternalChatID,
		&chat.ChatData,
		&chat.Feedback,
		&chat.Status,
		&chat.CreatedAt,
		&chat.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("error scanning chat: %w", err)
	}

	return &chat, nil
}

const getChatByID = `-- name: GetChatByID :one
SELECT id, chatbot_id, organization_id, interface_id, external_chat_id, chat_data, feedback, status, created_at, updated_at
FROM chats
WHERE id = $1 AND organization_id = $2;
`

func (s *PostgresStore) GetChatByID(ctx context.Context, id uuid.UUID, orgID uuid.UUID) (*models.Chat, error) {
	row := s.db.QueryRow(ctx, getChatByID, id, orgID)

	var chat models.Chat
	err := row.Scan(
		&chat.ID,
		&chat.ChatbotID,
		&chat.OrganizationID,
		&chat.InterfaceID,
		&chat.ExternalChatID,
		&chat.ChatData,
		&chat.Feedback,
		&chat.Status,
		&chat.CreatedAt,
		&chat.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("error scanning chat: %w", err)
	}

	return &chat, nil
}

const getChatByExternalID = `-- name: GetChatByExternalID :one
SELECT id, chatbot_id, organization_id, interface_id, external_chat_id, chat_data, feedback, status, created_at, updated_at
FROM chats
WHERE external_chat_id = $1 AND interface_id = $2 AND organization_id = $3;
`

func (s *PostgresStore) GetChatByExternalID(ctx context.Context, externalID string, interfaceID uuid.UUID, orgID uuid.UUID) (*models.Chat, error) {
	row := s.db.QueryRow(ctx, getChatByExternalID, externalID, interfaceID, orgID)

	var chat models.Chat
	err := row.Scan(
		&chat.ID,
		&chat.ChatbotID,
		&chat.OrganizationID,
		&chat.InterfaceID,
		&chat.ExternalChatID,
		&chat.ChatData,
		&chat.Feedback,
		&chat.Status,
		&chat.CreatedAt,
		&chat.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("error scanning chat: %w", err)
	}

	return &chat, nil
}

const listChatsByOrg = `-- name: ListChatsByOrg :many
SELECT id, chatbot_id, organization_id, interface_id, external_chat_id, chat_data, feedback, status, created_at, updated_at
FROM chats
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
`

func (s *PostgresStore) ListChatsByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]models.Chat, error) {
	rows, err := s.db.Query(ctx, listChatsByOrg, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying chats: %w", err)
	}
	defer rows.Close()

	var chats []models.Chat
	for rows.Next() {
		var chat models.Chat
		if err := rows.Scan(
			&chat.ID,
			&chat.ChatbotID,
			&chat.OrganizationID,
			&chat.InterfaceID,
			&chat.ExternalChatID,
			&chat.ChatData,
			&chat.Feedback,
			&chat.Status,
			&chat.CreatedAt,
			&chat.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning chat row: %w", err)
		}
		chats = append(chats, chat)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chat rows: %w", err)
	}

	return chats, nil
}

const listChatsByChatbot = `-- name: ListChatsByChatbot :many
SELECT id, chatbot_id, organization_id, interface_id, external_chat_id, chat_data, feedback, status, created_at, updated_at
FROM chats
WHERE chatbot_id = $1 AND organization_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
`

func (s *PostgresStore) ListChatsByChatbot(ctx context.Context, chatbotID, orgID uuid.UUID, limit, offset int) ([]models.Chat, error) {
	rows, err := s.db.Query(ctx, listChatsByChatbot, chatbotID, orgID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying chats: %w", err)
	}
	defer rows.Close()

	var chats []models.Chat
	for rows.Next() {
		var chat models.Chat
		if err := rows.Scan(
			&chat.ID,
			&chat.ChatbotID,
			&chat.OrganizationID,
			&chat.InterfaceID,
			&chat.ExternalChatID,
			&chat.ChatData,
			&chat.Feedback,
			&chat.Status,
			&chat.CreatedAt,
			&chat.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning chat row: %w", err)
		}
		chats = append(chats, chat)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chat rows: %w", err)
	}

	return chats, nil
}

// AddMessageToChat appends a new message to the chat's chat_data JSONB field.
func (s *PostgresStore) AddMessageToChat(ctx context.Context, chatID uuid.UUID, message models.ChatMessage, orgID uuid.UUID) error {
	// First, get the current chat to verify access and get current chat_data
	chat, err := s.GetChatByID(ctx, chatID, orgID)
	if err != nil {
		return fmt.Errorf("failed to retrieve chat: %w", err)
	}

	// Parse existing messages
	var messages []models.ChatMessage
	if err := json.Unmarshal(chat.ChatData, &messages); err != nil {
		return fmt.Errorf("failed to parse chat data: %w", err)
	}

	// Ensure the message has SentBy set to match Role if not already set
	if message.SentBy == "" {
		message.SentBy = message.Role
	}

	// Set Hide to 0 (visible) if not explicitly set
	// Hide == 1 means hidden, Hide == 0 means visible

	// Append new message
	messages = append(messages, message)

	// Marshal back to JSON
	updatedData, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("failed to marshal updated chat data: %w", err)
	}

	// Update the chat in the database
	const updateChatData = `
		UPDATE chats
		SET chat_data = $1, updated_at = NOW()
		WHERE id = $2 AND organization_id = $3;
	`

	tag, err := s.db.Exec(ctx, updateChatData, updatedData, chatID, orgID)
	if err != nil {
		return fmt.Errorf("failed to update chat data: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}

	return nil
}

// UpdateChatStatus updates the status of a chat.
func (s *PostgresStore) UpdateChatStatus(ctx context.Context, chatID uuid.UUID, status string, orgID uuid.UUID) error {
	// Validate status is one of the allowed values
	validStatuses := []string{"ACTIVE", "PROCESSING", "COMPLETED", "ERROR"}
	isValid := false
	for _, validStatus := range validStatuses {
		if status == validStatus {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid status: %s", status)
	}

	const updateStatus = `
		UPDATE chats
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND organization_id = $3;
	`

	tag, err := s.db.Exec(ctx, updateStatus, status, chatID, orgID)
	if err != nil {
		return fmt.Errorf("failed to update chat status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}

	return nil
}

// UpdateChatFeedback updates the feedback value of a chat.
func (s *PostgresStore) UpdateChatFeedback(ctx context.Context, chatID uuid.UUID, feedback int8, orgID uuid.UUID) error {
	// Validate feedback is one of the allowed values (-1, 0, 1)
	if feedback < -1 || feedback > 1 {
		return fmt.Errorf("invalid feedback value: %d, must be -1, 0, or 1", feedback)
	}

	const updateFeedback = `
		UPDATE chats
		SET feedback = $1, updated_at = NOW()
		WHERE id = $2 AND organization_id = $3;
	`

	tag, err := s.db.Exec(ctx, updateFeedback, feedback, chatID, orgID)
	if err != nil {
		return fmt.Errorf("failed to update chat feedback: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}

	return nil
}
