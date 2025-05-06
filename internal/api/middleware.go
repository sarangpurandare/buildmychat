package api

import (
	"buildmychat-backend/internal/auth" // Use the definition from auth package
	"buildmychat-backend/pkg/httputil"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// --- JWT Middleware ---

// JwtAuthMiddleware verifies the JWT token from the Authorization header.
// If valid, it injects UserID and OrgID into the request context.
func JwtAuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Println("Auth Middleware: Missing Authorization header")
				httputil.RespondError(w, http.StatusUnauthorized, "Authorization header required")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				log.Printf("Auth Middleware: Malformed Authorization header: %s", authHeader)
				httputil.RespondError(w, http.StatusUnauthorized, "Malformed Authorization header (Expected: Bearer <token>)")
				return
			}

			tokenString := parts[1]
			claims := &auth.CustomClaims{} // Use CustomClaims from the auth package

			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				// Validate the signing algorithm
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				// Return the secret key for validation
				return []byte(jwtSecret), nil
			})

			if err != nil {
				log.Printf("Auth Middleware: Error parsing token: %v", err)
				if errors.Is(err, jwt.ErrTokenExpired) {
					httputil.RespondError(w, http.StatusUnauthorized, "Token has expired")
				} else if errors.Is(err, jwt.ErrTokenMalformed) {
					httputil.RespondError(w, http.StatusUnauthorized, "Malformed token")
				} else {
					httputil.RespondError(w, http.StatusUnauthorized, "Invalid token")
				}
				return
			}

			if !token.Valid {
				log.Println("Auth Middleware: Token is present but invalid")
				httputil.RespondError(w, http.StatusUnauthorized, "Invalid token")
				return
			}

			// Token is valid, extract custom claims
			userID := claims.UserID
			orgID := claims.OrgID

			if userID == uuid.Nil || orgID == uuid.Nil {
				log.Printf("Auth Middleware: Missing UserID (%s) or OrgID (%s) in valid token claims", userID, orgID)
				httputil.RespondError(w, http.StatusUnauthorized, "Invalid token claims (missing IDs)")
				return
			}

			// Add user info to context
			ctx := context.WithValue(r.Context(), auth.UserIDKey, userID)
			ctx = context.WithValue(ctx, auth.OrgIDKey, orgID)

			// Call the next handler in the chain with the enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
