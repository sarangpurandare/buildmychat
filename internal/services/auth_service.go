package services

import (
	"buildmychat-backend/internal/auth"
	"buildmychat-backend/internal/config"
	"buildmychat-backend/internal/models"
	"buildmychat-backend/internal/store"
	"context"
	"errors"
	"fmt"
	"log" // Or your preferred logger
	"strings"

	"github.com/google/uuid"
)

// Custom errors for auth service
var (
	ErrUserAlreadyExists  = errors.New("user with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrHashingPassword    = errors.New("failed to hash password")
	ErrCreatingToken      = errors.New("failed to create access token")
	ErrCreatingOrgOrUser  = errors.New("failed to create organization or user")
	ErrValidation         = errors.New("input validation failed") // Generic validation error
)

type AuthService struct {
	store store.Store
	cfg   *config.Config
}

func NewAuthService(s store.Store, cfg *config.Config) *AuthService {
	return &AuthService{
		store: s,
		cfg:   cfg,
	}
}

// Signup creates a new organization and user.
func (s *AuthService) Signup(ctx context.Context, email, password string) (*models.User, error) {
	// Basic validation
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil, fmt.Errorf("%w: email and password cannot be empty", ErrValidation)
	}
	// TODO: Add more robust email validation (e.g., regex) if needed

	// Check if user already exists
	_, err := s.store.GetUserByEmail(ctx, email)
	if err == nil {
		// User found, return conflict error
		return nil, ErrUserAlreadyExists
	}
	if !errors.Is(err, store.ErrNotFound) {
		// Different error occurred during lookup
		log.Printf("Error checking user existence for %s: %v", email, err)
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	// User does not exist (store.ErrNotFound received), proceed.

	// Hash password
	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		log.Printf("Error hashing password for %s: %v", email, err)
		return nil, ErrHashingPassword
	}

	// --- Database Transaction (Recommended for multi-step creation) ---
	// For simplicity in MVP, we're doing sequential creates.
	// In a real app, wrap Org + User creation in a DB transaction.
	// tx, err := s.store.BeginTx(ctx)
	// if err != nil { /* handle error */ }
	// defer tx.Rollback() // Rollback if anything fails
	// ... use tx for CreateOrganization and CreateUser ...
	// tx.Commit()
	// ------------------------------------------------------------------

	// Create Organization
	org := &models.Organization{
		ID:   uuid.New(),
		Name: fmt.Sprintf("%s's Workspace", email), // Default name
		// CreatedAt/UpdatedAt typically set by DB or ORM
	}
	if err := s.store.CreateOrganization(ctx, org); err != nil {
		log.Printf("Error creating organization for %s: %v", email, err)
		return nil, fmt.Errorf("%w: creating organization failed: %v", ErrCreatingOrgOrUser, err)
	}

	// Create User
	user := &models.User{
		ID:             uuid.New(),
		OrganizationID: org.ID,
		Email:          email,
		HashedPassword: hashedPassword,
		// CreatedAt/UpdatedAt typically set by DB or ORM
	}
	if err := s.store.CreateUser(ctx, user); err != nil {
		// Attempt to clean up organization if user creation fails? Maybe not critical for MVP.
		log.Printf("Error creating user for %s (OrgID: %s): %v", email, org.ID, err)
		// Consider attempting to delete the created organization here if critical
		return nil, fmt.Errorf("%w: creating user failed: %v", ErrCreatingOrgOrUser, err)
	}

	log.Printf("Successfully signed up user %s (ID: %s) in Org %s (ID: %s)", email, user.ID, org.Name, org.ID)
	return user, nil
}

// Login verifies user credentials and returns an access token and user info.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, *models.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return "", nil, ErrInvalidCredentials // Basic check before hitting DB
	}

	// Get user by email
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", nil, ErrInvalidCredentials // Don't reveal if user exists or password is wrong
		}
		log.Printf("Error retrieving user %s during login: %v", email, err)
		return "", nil, fmt.Errorf("failed to retrieve user: %w", err)
	}

	// Check password
	if !auth.CheckPasswordHash(password, user.HashedPassword) {
		return "", nil, ErrInvalidCredentials // Password mismatch
	}

	// Generate JWT token using expiration from config
	token, err := auth.NewAccessToken(user.ID, user.OrganizationID, s.cfg.JWTSecret, s.cfg.TokenExpiration)
	if err != nil {
		log.Printf("Error generating JWT for user %s (ID: %s): %v", email, user.ID, err)
		return "", nil, ErrCreatingToken
	}

	// Optional: Update LastLoginAt timestamp (add UpdateUserLastLogin to store interface)
	// if err := s.store.UpdateUserLastLogin(ctx, user.ID); err != nil {
	// 	 log.Printf("Warning: Failed to update last login for user %s: %v", user.ID, err)
	// }

	log.Printf("Successfully logged in user %s (ID: %s)", email, user.ID)
	return token, user, nil
}
