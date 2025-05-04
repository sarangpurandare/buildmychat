package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// getOrgIDFromContext extracts the organization ID from the context.
func getOrgIDFromContext(ctx context.Context) (uuid.UUID, error) {
	// In a real implementation, this would get the organization ID from JWT claims
	// For now, we'll use a placeholder method that returns a fixed org ID for testing
	orgID, ok := ctx.Value("organization_id").(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("organization ID not found in context")
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
