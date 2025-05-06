package handlers

import (
	"buildmychat-backend/internal/auth"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// getOrgIDFromContext retrieves the organization ID from the context
// using the helper function from the auth package.
func getOrgIDFromContext(ctx context.Context) (uuid.UUID, error) {
	orgID, ok := auth.GetOrgIDFromContext(ctx)
	if !ok {
		return uuid.Nil, fmt.Errorf("organization ID not found in context or not a UUID")
	}
	return orgID, nil
}

// respondWithJSON writes a JSON response.
func respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If encoding fails, return a simple error message
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

// respondWithError writes an error response in JSON format.
func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	respondWithJSON(w, statusCode, map[string]string{"error": message})
}
