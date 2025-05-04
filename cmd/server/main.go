package main

import (
	"buildmychat-backend/internal/api"
	api_models "buildmychat-backend/internal/models" // For service types

	// "buildmychat-backend/internal/auth" // Not directly needed here
	"buildmychat-backend/internal/config"
	"buildmychat-backend/internal/crypto" // Import crypto package
	"buildmychat-backend/internal/handlers"
	"buildmychat-backend/internal/integrations" // Import integrations package
	"buildmychat-backend/internal/services"
	"buildmychat-backend/internal/store/postgres"
	"context" // Import cipher package
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log.Println("Starting BuildMyChat Backend...")

	// 1. Load Configuration
	cfg, err := config.LoadConfig() // Using the function from internal/config
	if err != nil {
		log.Fatalf("FATAL: Failed to load configuration: %v", err)
	}
	log.Println("Configuration loaded successfully.")

	// 2. Initialize Database Connection Pool
	// Use context.Background() for initial setup, but request-scoped contexts later.
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second) // Timeout for initial connection
	defer dbCancel()

	dbpool, err := pgxpool.New(dbCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("FATAL: Unable to create database connection pool: %v\n", err)
	}
	defer dbpool.Close() // Ensure pool is closed on exit

	// Ping DB to verify connection
	if err := dbpool.Ping(dbCtx); err != nil {
		log.Fatalf("FATAL: Unable to ping database: %v\n", err)
	}
	log.Println("Database connection pool established and pinged successfully.")

	// 3. Initialize Dependencies (Store, Services, Handlers)
	pgStore := postgres.NewPostgresStore(dbpool)
	log.Println("Postgres store initialized.")

	// --- Create AEAD Cipher for Encryption ---
	aead, err := crypto.NewAESGCM(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("FATAL: Failed to create AES-GCM cipher: %v", err)
	}
	log.Println("AES-GCM cipher initialized.")

	// --- Initialize Integration Registry ---
	intRegistry := integrations.NewRegistry()
	notionIntegration := integrations.NewNotionIntegration()
	slackIntegration := integrations.NewSlackIntegration()
	intRegistry.Register(string(api_models.ServiceTypeNotion), notionIntegration)
	intRegistry.Register(string(api_models.ServiceTypeSlack), slackIntegration)
	log.Println("IntegrationRegistry initialized and populated.")

	// --- Initialize Services ---
	// Only Auth service for now
	authService := services.NewAuthService(pgStore, cfg)
	log.Println("AuthService initialized.")
	credentialService := services.NewCredentialsService(pgStore, aead, intRegistry) // Inject registry
	log.Println("CredentialsService initialized.")
	kbService := services.NewKBService(pgStore)
	log.Println("KBService initialized.")
	interfaceService := services.NewInterfaceService(pgStore)
	log.Println("InterfaceService initialized.")
	chatbotService := services.NewChatbotService(pgStore)
	log.Println("ChatbotService initialized.")
	// ... Initialize other services here as they are created ...

	// --- Initialize Handlers ---
	authHandler := handlers.NewAuthHandler(authService)
	log.Println("AuthHandler initialized.")
	credentialHandler := handlers.NewCredentialsHandler(credentialService)
	log.Println("CredentialsHandler initialized.")
	kbHandler := handlers.NewKBHandler(kbService)
	log.Println("KBHandler initialized.")
	interfaceHandler := handlers.NewInterfaceHandler(interfaceService)
	log.Println("InterfaceHandler initialized.")
	chatbotHandler := handlers.NewChatbotHandlers(chatbotService)
	log.Println("ChatbotHandler initialized.")
	// ... Initialize other handlers here ...

	// 4. Setup Router & Inject Dependencies
	routerDeps := api.RouterDependencies{
		AuthHandler:        authHandler,
		CredentialsHandler: credentialHandler,
		KBHandler:          kbHandler,        // Add KB handler
		InterfaceHandler:   interfaceHandler, // Add Interface handler
		ChatbotHandler:     chatbotHandler,   // Add Chatbot handler
		Config:             cfg,
	}
	router := api.NewRouter(routerDeps) // Use the NewRouter function from internal/api
	log.Println("HTTP router configured.")

	// 5. Configure and Start HTTP Server
	server := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
		// Production hardening: Set timeouts to avoid Slowloris attacks
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Channel to listen for OS signals for graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	// Run server in a goroutine so it doesn't block
	go func() {
		log.Printf("Server starting and listening on port %s", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("FATAL: Could not listen on %s: %v\n", cfg.HTTPPort, err)
		}
		log.Println("Server listener routine stopped.")
	}()

	// Wait for interrupt signal
	<-stopChan
	log.Println("Shutdown signal received, initiating graceful shutdown...")

	// Create a deadline context for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("WARN: Server graceful shutdown failed: %v", err)
		log.Fatal("Forcing shutdown due to error.") // Or handle more gracefully
	}

	log.Println("Server shutdown complete.")
}
