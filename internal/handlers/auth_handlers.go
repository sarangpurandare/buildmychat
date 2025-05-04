package handlers

import (
	api_models "buildmychat-backend/internal/models"
	db_models "buildmychat-backend/internal/models"
	"buildmychat-backend/internal/services"
	"buildmychat-backend/pkg/httputil"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

// AuthService defines the interface expected from the auth service.
// This promotes loose coupling and testability.
type AuthService interface {
	Signup(ctx context.Context, email, password string) (*db_models.User, error)
	Login(ctx context.Context, email, password string) (string, *db_models.User, error)
}

type AuthHandler struct {
	authService AuthService
}

func NewAuthHandler(authSvc AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authSvc,
	}
}

// HandleSignup handles the POST /v1/auth/signup request.
func (h *AuthHandler) HandleSignup(w http.ResponseWriter, r *http.Request) {
	var req api_models.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	// Basic validation in handler (can be moved to service or dedicated validator)
	if req.Email == "" || req.Password == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	user, err := h.authService.Signup(r.Context(), req.Email, req.Password)
	if err != nil {
		log.Printf("Signup handler failed for email %s: %v", req.Email, err)
		// Error Mapping: Map service errors to HTTP status codes
		switch {
		case errors.Is(err, services.ErrUserAlreadyExists):
			httputil.RespondError(w, http.StatusConflict, err.Error()) // 409
		case errors.Is(err, services.ErrValidation):
			httputil.RespondError(w, http.StatusBadRequest, err.Error()) // 400
		case errors.Is(err, services.ErrHashingPassword):
			fallthrough // Treat hashing, token, db errors as internal server errors
		case errors.Is(err, services.ErrCreatingOrgOrUser):
			fallthrough
		default:
			httputil.RespondError(w, http.StatusInternalServerError, "Signup failed due to an internal error") // 500
		}
		return
	}

	// Return minimal user info on signup
	resp := api_models.UserResponse{
		ID:             user.ID,
		Email:          user.Email,
		OrganizationID: user.OrganizationID,
	}
	httputil.RespondJSON(w, http.StatusCreated, resp) // 201 Created
}

// HandleLogin handles the POST /v1/auth/login request.
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req api_models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	if req.Email == "" || req.Password == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	token, user, err := h.authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		log.Printf("Login handler failed for email %s: %v", req.Email, err)
		// Error Mapping
		switch {
		case errors.Is(err, services.ErrInvalidCredentials):
			httputil.RespondError(w, http.StatusUnauthorized, err.Error()) // 401
		case errors.Is(err, services.ErrCreatingToken):
			fallthrough // Treat token creation or other unexpected errors as internal
		default:
			httputil.RespondError(w, http.StatusInternalServerError, "Login failed due to an internal error") // 500
		}
		return
	}

	resp := api_models.AuthResponse{
		AccessToken: token,
		User: api_models.UserResponse{ // Map db.User to api.UserResponse
			ID:             user.ID,
			Email:          user.Email,
			OrganizationID: user.OrganizationID,
		},
	}
	httputil.RespondJSON(w, http.StatusOK, resp) // 200 OK
}
