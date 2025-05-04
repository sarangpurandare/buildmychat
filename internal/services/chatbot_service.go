package services

import (
	"buildmychat-backend/internal/models"
	"buildmychat-backend/internal/store"
	"context"
	"fmt" // Import fmt

	"github.com/google/uuid"
)

// ChatbotService handles business logic related to chatbots.
type ChatbotService struct {
	store store.Store
}

// NewChatbotService creates a new ChatbotService.
func NewChatbotService(store store.Store) *ChatbotService {
	return &ChatbotService{store: store}
}

// mapChatbotToResponse converts a DB chatbot model to an API response DTO.
func mapChatbotToResponse(dbChatbot models.Chatbot) models.ChatbotResponse {
	return models.ChatbotResponse{
		ID:             dbChatbot.ID,
		OrganizationID: dbChatbot.OrganizationID,
		Name:           dbChatbot.Name,
		SystemPrompt:   dbChatbot.SystemPrompt,
		IsActive:       dbChatbot.IsActive,
		ChatCount:      dbChatbot.ChatCount,
		LLMModel:       dbChatbot.LLMModel,
		Configuration:  &dbChatbot.Configuration, // Pass pointer to RawMessage
		CreatedAt:      dbChatbot.CreatedAt,
		UpdatedAt:      dbChatbot.UpdatedAt,
	}
}

// mapChatbotWithMappingsToResponse converts a DB chatbot model to an API response DTO with nested resources.
func (s *ChatbotService) mapChatbotWithMappingsToResponse(ctx context.Context, dbChatbot models.Chatbot) (models.ChatbotResponse, error) {
	resp := mapChatbotToResponse(dbChatbot)

	// Get the mappings
	mappings, err := s.store.GetChatbotMappings(ctx, dbChatbot.ID, dbChatbot.OrganizationID)
	if err != nil && err != store.ErrNotFound {
		return resp, fmt.Errorf("failed to get mappings: %w", err)
	}

	if mappings != nil {
		resp.KnowledgeBases = mappings.KnowledgeBases
		resp.Interfaces = mappings.Interfaces
	}

	return resp, nil
}

// CreateChatbot creates a new chatbot for a given organization.
func (s *ChatbotService) CreateChatbot(ctx context.Context, orgID uuid.UUID, req models.CreateChatbotRequest) (*models.ChatbotResponse, error) {
	params := store.CreateChatbotParams{
		OrganizationID: orgID,
		Name:           req.Name, // Pass pointer directly
		SystemPrompt:   req.SystemPrompt,
		LLMModel:       req.LLMModel,
		Configuration:  req.Configuration, // Pass pointer directly
	}

	dbChatbot, err := s.store.CreateChatbot(ctx, params)
	if err != nil {
		// Handle specific DB errors if needed (e.g., unique constraint)
		return nil, fmt.Errorf("failed to create chatbot in store: %w", err)
	}

	resp := mapChatbotToResponse(dbChatbot)
	return &resp, nil
}

// GetChatbotByID retrieves a specific chatbot by its ID for a given organization.
func (s *ChatbotService) GetChatbotByID(ctx context.Context, orgID, chatbotID uuid.UUID) (*models.ChatbotResponse, error) {
	dbChatbot, err := s.store.GetChatbotByID(ctx, chatbotID, orgID)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, err // Propagate ErrNotFound
		}
		return nil, fmt.Errorf("failed to get chatbot from store: %w", err)
	}

	resp, err := s.mapChatbotWithMappingsToResponse(ctx, dbChatbot)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare chatbot response: %w", err)
	}

	return &resp, nil
}

// ListChatbots retrieves all chatbots for a given organization.
func (s *ChatbotService) ListChatbots(ctx context.Context, orgID uuid.UUID) (*models.ListChatbotsResponse, error) {
	dbChatbots, err := s.store.ListChatbots(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list chatbots from store: %w", err)
	}

	responseChatbots := make([]models.ChatbotResponse, len(dbChatbots))
	for i, dbChatbot := range dbChatbots {
		respChatbot, err := s.mapChatbotWithMappingsToResponse(ctx, dbChatbot)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare chatbot response at index %d: %w", i, err)
		}
		responseChatbots[i] = respChatbot
	}

	return &models.ListChatbotsResponse{Chatbots: responseChatbots}, nil
}

// UpdateChatbot updates an existing chatbot.
func (s *ChatbotService) UpdateChatbot(ctx context.Context, orgID, chatbotID uuid.UUID, req models.UpdateChatbotRequest) (*models.ChatbotResponse, error) {
	params := store.UpdateChatbotParams{
		ID:             chatbotID,
		OrganizationID: orgID,
		Name:           req.Name,
		SystemPrompt:   req.SystemPrompt,
		LLMModel:       req.LLMModel,
		Configuration:  req.Configuration,
	}

	dbChatbot, err := s.store.UpdateChatbot(ctx, params)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, err // Propagate ErrNotFound
		}
		// Handle other potential errors (e.g., validation errors if added later)
		return nil, fmt.Errorf("failed to update chatbot in store: %w", err)
	}

	resp := mapChatbotToResponse(dbChatbot)
	return &resp, nil
}

// UpdateChatbotStatus activates or deactivates a chatbot.
func (s *ChatbotService) UpdateChatbotStatus(ctx context.Context, orgID, chatbotID uuid.UUID, req models.UpdateChatbotStatusRequest) error {
	err := s.store.UpdateChatbotStatus(ctx, chatbotID, orgID, req.IsActive)
	if err != nil {
		if err == store.ErrNotFound {
			return err // Propagate ErrNotFound
		}
		return fmt.Errorf("failed to update chatbot status in store: %w", err)
	}
	return nil
}

// DeleteChatbot deletes a chatbot.
func (s *ChatbotService) DeleteChatbot(ctx context.Context, orgID, chatbotID uuid.UUID) error {
	err := s.store.DeleteChatbot(ctx, chatbotID, orgID)
	if err != nil {
		if err == store.ErrNotFound {
			return err // Propagate ErrNotFound
		}
		return fmt.Errorf("failed to delete chatbot from store: %w", err)
	}
	return nil
}

// GetChatbotMappings retrieves all knowledge bases and interfaces mapped to a chatbot.
func (s *ChatbotService) GetChatbotMappings(ctx context.Context, orgID, chatbotID uuid.UUID) (*models.ChatbotMappingsResponse, error) {
	mappings, err := s.store.GetChatbotMappings(ctx, chatbotID, orgID)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, err // Propagate ErrNotFound
		}
		return nil, fmt.Errorf("failed to get chatbot mappings from store: %w", err)
	}
	return mappings, nil
}

// AddKnowledgeBase adds a Knowledge Base to a chatbot.
func (s *ChatbotService) AddKnowledgeBase(ctx context.Context, orgID, chatbotID, kbID uuid.UUID) error {
	err := s.store.AddKnowledgeBaseMapping(ctx, chatbotID, kbID, orgID)
	if err != nil {
		if err == store.ErrNotFound {
			return err // Propagate ErrNotFound
		}
		return fmt.Errorf("failed to add knowledge base mapping in store: %w", err)
	}
	return nil
}

// RemoveKnowledgeBase removes a Knowledge Base from a chatbot.
func (s *ChatbotService) RemoveKnowledgeBase(ctx context.Context, orgID, chatbotID, kbID uuid.UUID) error {
	err := s.store.RemoveKnowledgeBaseMapping(ctx, chatbotID, kbID, orgID)
	if err != nil {
		if err == store.ErrNotFound {
			return err // Propagate ErrNotFound
		}
		return fmt.Errorf("failed to remove knowledge base mapping in store: %w", err)
	}
	return nil
}

// AddInterface adds an Interface to a chatbot.
func (s *ChatbotService) AddInterface(ctx context.Context, orgID, chatbotID, interfaceID uuid.UUID) error {
	err := s.store.AddInterfaceMapping(ctx, chatbotID, interfaceID, orgID)
	if err != nil {
		if err == store.ErrNotFound {
			return err // Propagate ErrNotFound
		}
		return fmt.Errorf("failed to add interface mapping in store: %w", err)
	}
	return nil
}

// RemoveInterface removes an Interface from a chatbot.
func (s *ChatbotService) RemoveInterface(ctx context.Context, orgID, chatbotID, interfaceID uuid.UUID) error {
	err := s.store.RemoveInterfaceMapping(ctx, chatbotID, interfaceID, orgID)
	if err != nil {
		if err == store.ErrNotFound {
			return err // Propagate ErrNotFound
		}
		return fmt.Errorf("failed to remove interface mapping in store: %w", err)
	}
	return nil
}
