package integrations

import (
	integration_models "buildmychat-backend/internal/models/integrations"
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// Integration defines the standard interface for all external service integrations.
type Integration interface {
	// ValidateConfig parses and validates the specific configuration JSON for this integration type.
	// It should return a specific validation error if the config is invalid.
	ValidateConfig(configJSON json.RawMessage) error

	// TestConnection attempts to connect to the external service using the provided decrypted credentials.
	// It returns a result indicating success/failure and an optional message.
	TestConnection(ctx context.Context, decryptedCreds integration_models.DecryptedCredentials) (*integration_models.TestConnectionResult, error)

	// GetCredentialSchema returns the expected structure (as an empty struct instance)
	// for the credentials required by this integration, used for unmarshalling.
	GetCredentialSchema() interface{}
}

// Registry holds the mapping between service types and their Integration implementations.
type Registry struct {
	integrations map[string]Integration
}

// NewRegistry creates a new integration registry.
func NewRegistry() *Registry {
	return &Registry{
		integrations: make(map[string]Integration),
	}
}

// Register adds an integration implementation to the registry.
func (r *Registry) Register(serviceType string, integration Integration) {
	if _, exists := r.integrations[serviceType]; exists {
		log.Printf("WARN [IntegrationRegistry] Service type '%s' is already registered. Overwriting.", serviceType)
	}
	r.integrations[serviceType] = integration
	log.Printf("[IntegrationRegistry] Registered integration for service type: %s", serviceType)
}

// Get retrieves an integration implementation from the registry by service type.
func (r *Registry) Get(serviceType string) (Integration, error) {
	integration, exists := r.integrations[serviceType]
	if !exists {
		return nil, fmt.Errorf("no integration registered for service type: %s", serviceType)
	}
	return integration, nil
}

// MustGet retrieves an integration implementation, panicking if not found.
// Useful during initialization if an integration is expected to be present.
func (r *Registry) MustGet(serviceType string) Integration {
	integration, err := r.Get(serviceType)
	if err != nil {
		panic(fmt.Sprintf("FATAL [IntegrationRegistry] %v", err))
	}
	return integration
}
