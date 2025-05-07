package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// RespondWithError responds with an error message.
func RespondWithError(w http.ResponseWriter, code int, message string) {
	RespondWithJSON(w, code, map[string]string{"error": message})
}

// RespondWithJSON responds with a JSON payload.
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// GetOrgIDFromContext extracts the organization ID from the context.
func GetOrgIDFromContext(ctx context.Context) (uuid.UUID, error) {
	orgIDVal := ctx.Value("organization_id")
	if orgIDVal == nil {
		return uuid.Nil, fmt.Errorf("organization_id not found in context")
	}

	orgID, ok := orgIDVal.(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("organization_id in context is not a valid UUID")
	}

	return orgID, nil
}
