package auth

import (
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// --- Context Keys ---

// contextKey is a custom type used for context keys to avoid collisions.
type contextKey string

const (
	UserIDKey contextKey = "userID"
	OrgIDKey  contextKey = "orgID"
)

// --- JWT Claims ---

// CustomClaims includes standard JWT claims plus our custom ones.
// Match this with the claims struct in api/middleware.go
type CustomClaims struct {
	UserID uuid.UUID `json:"user_id"`
	OrgID  uuid.UUID `json:"org_id"`
	jwt.RegisteredClaims
}

// NewAccessToken generates a new JWT access token.
func NewAccessToken(userID uuid.UUID, orgID uuid.UUID, jwtSecret string, expiration time.Duration) (string, error) {
	// Create the claims
	claims := CustomClaims{
		UserID: userID,
		OrgID:  orgID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "buildmychat-backend", // Optional: Identify the issuer
			Subject:   userID.String(),       // Optional: Subject identifies the principal (user)
			// ID:        // Optional: Unique identifier for the token (jti)
		},
	}

	// Create the token using your claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	signedToken, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		log.Printf("Error signing JWT token for UserID %s: %v", userID, err)
		return "", err
	}

	return signedToken, nil
}

// TODO: Add function to ParseAndValidateToken if needed elsewhere besides middleware
