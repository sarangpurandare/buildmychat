package api

import (
	"buildmychat-backend/internal/config"
	"buildmychat-backend/internal/handlers"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// RouterDependencies holds all the dependencies required by the router setup,
// primarily handlers and configuration.
type RouterDependencies struct {
	AuthHandler         *handlers.AuthHandler
	CredentialsHandler  *handlers.CredentialsHandler
	KBHandler           *handlers.KBHandler
	InterfaceHandler    *handlers.InterfaceHandler
	ChatbotHandler      *handlers.ChatbotHandlers
	ChatHandler         *handlers.ChatHandlers
	SlackWebhookHandler *handlers.SlackWebhookHandlers
	// OrgHandler        *handlers.OrgHandler
	// NodeHandler       *handlers.NodeHandler
	// WebhookHandler    *handlers.WebhookHandler
	// BillingHandler    *handlers.BillingHandler
	Config *config.Config
}

// NewRouter creates and configures the main Chi router for the application.
func NewRouter(deps RouterDependencies) *chi.Mux {
	r := chi.NewRouter()

	// --- Base Middleware Stack ---
	r.Use(middleware.RequestID)                 // Inject request ID into context
	r.Use(middleware.RealIP)                    // Use X-Forwarded-For or X-Real-IP
	r.Use(middleware.Logger)                    // Log requests (consider a structured logger)
	r.Use(middleware.Recoverer)                 // Recover from panics, return 500
	r.Use(middleware.Timeout(60 * time.Second)) // Set a request timeout

	// --- CORS Configuration ---
	// Adjust AllowedOrigins for your frontend deployment(s)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:5173", "https://*.vercel.app"}, // Add your frontend dev/prod URLs
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// --- Public Routes (No JWT Required) ---
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}) // Moved health check here

	r.Route("/v1/auth", func(r chi.Router) {
		if deps.AuthHandler == nil {
			panic("AuthHandler dependency is nil in router setup")
		}
		r.Post("/signup", deps.AuthHandler.HandleSignup)
		r.Post("/login", deps.AuthHandler.HandleLogin)
	})

	// --- Public Slack Event Webhook ---
	// This needs to be public for Slack to send events (including initial URL verification).
	// Signature verification within the handler will secure it.
	if deps.SlackWebhookHandler != nil {
		r.Route("/slack-events", func(r chi.Router) {
			r.Post("/{chatbotID}", deps.SlackWebhookHandler.HandleSlackEvent)
		})
	} else {
		log.Println("WARN: SlackWebhookHandler dependency is nil, skipping /v1/slack-events routes.")
	}

	// --- Authenticated Routes (JWT Required) ---
	r.Route("/v1", func(r chi.Router) {
		// Apply JWT Authentication Middleware
		r.Use(JwtAuthMiddleware(deps.Config.JWTSecret))

		// --- Mount Protected Handler Groups Here ---

		// Example: Organization/User routes
		// if deps.OrgHandler != nil {
		// 	 r.Get("/me", deps.OrgHandler.HandleGetMe)
		// 	 r.Get("/organization", deps.OrgHandler.HandleGetOrganization)
		// 	 r.Patch("/organization", deps.OrgHandler.HandleUpdateOrganization)
		// }

		// --- Mount Credentials Routes ---
		if deps.CredentialsHandler != nil {
			r.Route("/credentials", func(r chi.Router) {
				r.Post("/", deps.CredentialsHandler.HandleCreateCredential)
				r.Get("/", deps.CredentialsHandler.HandleListCredentials)
				r.Get("/{credentialID}", deps.CredentialsHandler.HandleGetCredential)
				r.Delete("/{credentialID}", deps.CredentialsHandler.HandleDeleteCredential)
				r.Post("/{credentialID}/test", deps.CredentialsHandler.HandleTestCredential)
			})
		} else {
			log.Println("WARN: CredentialsHandler dependency is nil, skipping /v1/credentials routes.")
		}

		// --- Mount Knowledge Base Routes ---
		if deps.KBHandler != nil {
			r.Route("/knowledge-bases", func(r chi.Router) {
				r.Post("/", deps.KBHandler.HandleCreateKnowledgeBase)
				r.Get("/", deps.KBHandler.HandleListKnowledgeBases)
				r.Get("/{kbID}", deps.KBHandler.HandleGetKnowledgeBase)
				r.Put("/{kbID}", deps.KBHandler.HandleUpdateKnowledgeBase)
				r.Delete("/{kbID}", deps.KBHandler.HandleDeleteKnowledgeBase)
			})
		} else {
			log.Println("WARN: KBHandler dependency is nil, skipping /v1/knowledge-bases routes.")
		}

		// --- Mount Interface Routes ---
		if deps.InterfaceHandler != nil {
			r.Route("/interfaces", func(r chi.Router) {
				r.Post("/", deps.InterfaceHandler.HandleCreateInterface)
				r.Get("/", deps.InterfaceHandler.HandleListInterfaces)
				r.Get("/{interfaceID}", deps.InterfaceHandler.HandleGetInterface)
				r.Put("/{interfaceID}", deps.InterfaceHandler.HandleUpdateInterface)
				r.Delete("/{interfaceID}", deps.InterfaceHandler.HandleDeleteInterface)
			})
		} else {
			log.Println("WARN: InterfaceHandler dependency is nil, skipping /v1/interfaces routes.")
		}

		// --- Mount Chatbot Routes ---
		if deps.ChatbotHandler != nil {
			r.Route("/chatbots", func(r chi.Router) {
				r.Post("/", deps.ChatbotHandler.CreateChatbot)
				r.Get("/", deps.ChatbotHandler.ListChatbots)
				r.Get("/{chatbotID}", deps.ChatbotHandler.GetChatbotByID)
				r.Put("/{chatbotID}", deps.ChatbotHandler.UpdateChatbot)
				r.Patch("/{chatbotID}/status", deps.ChatbotHandler.UpdateChatbotStatus)
				r.Delete("/{chatbotID}", deps.ChatbotHandler.DeleteChatbot)

				// Chatbot mappings
				r.Get("/{chatbotID}/mappings", deps.ChatbotHandler.GetChatbotMappings)
				r.Post("/{chatbotID}/knowledge-bases", deps.ChatbotHandler.AddKnowledgeBase)
				r.Delete("/{chatbotID}/knowledge-bases/{kbID}", deps.ChatbotHandler.RemoveKnowledgeBase)
				r.Post("/{chatbotID}/interfaces", deps.ChatbotHandler.AddInterface)
				r.Delete("/{chatbotID}/interfaces/{interfaceID}", deps.ChatbotHandler.RemoveInterface)
			})
		} else {
			log.Println("WARN: ChatbotHandler dependency is nil, skipping /v1/chatbots routes.")
		}

		// --- Mount Chat Routes ---
		if deps.ChatHandler != nil {
			r.Route("/chats", func(r chi.Router) {
				r.Post("/", deps.ChatHandler.HandleCreateChat)
				r.Get("/", deps.ChatHandler.HandleListChats)
				r.Get("/{chatID}", deps.ChatHandler.HandleGetChatByID)

				// Message APIs
				r.Post("/{chatID}/messages/user", deps.ChatHandler.HandleAddUserMessage)
				r.Post("/{chatID}/messages/assistant", deps.ChatHandler.HandleAddAssistantMessage)
			})
		} else {
			log.Println("WARN: ChatHandler dependency is nil, skipping /v1/chats routes.")
		}

		// Example: Chat routes
		// if deps.ChatHandler != nil {
		// 	 r.Route("/chats", func(r chi.Router) {
		// 		 r.Post("/", deps.ChatHandler.HandleCreateChat)
		// 		 r.Get("/", deps.ChatHandler.HandleListChats)
		// 		 r.Post("/{chat_id}/messages", deps.ChatHandler.HandleSendMessage)
		// 		 // ... other chat routes /{chat_id}
		// 	 })
		// }

		// Example: Billing placeholder
		// if deps.BillingHandler != nil {
		// 	 r.Get("/billing/status", deps.BillingHandler.HandleGetBillingStatus)
		// }
	})

	return r
}
