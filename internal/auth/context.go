package auth

import (
	"context"

	"github.com/google/uuid"
)

// --- Context Helper Functions ---

// GetUserIDFromContext retrieves the UserID (uuid.UUID) from the request context.
// Returns the ID and true if found, otherwise uuid.Nil and false.
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

// GetOrgIDFromContext retrieves the OrgID (uuid.UUID) from the request context.
// Returns the ID and true if found, otherwise uuid.Nil and false.
func GetOrgIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	orgID, ok := ctx.Value(OrgIDKey).(uuid.UUID)
	return orgID, ok
}
