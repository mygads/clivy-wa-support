package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/handlers"
	"genfity-wa-support/middleware"
	"genfity-wa-support/services"
	"genfity-wa-support/worker"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, using system environment variables")
	} else {
		log.Println("✅ .env file loaded successfully")
	}

	// Debug: Print critical environment variables
	log.Printf("🔧 DATA_ACCESS_MODE: %s", os.Getenv("DATA_ACCESS_MODE"))
	log.Printf("🔧 TRANSACTIONAL_API_URL: %s", os.Getenv("TRANSACTIONAL_API_URL"))
	log.Printf("🔧 INTERNAL_API_KEY: %s", func() string {
		key := os.Getenv("INTERNAL_API_KEY")
		if len(key) > 10 {
			return key[:10] + "..."
		}
		return key
	}())

	// Initialize database
	database.InitDatabase()

	// Initialize data provider (API or Direct DB mode)
	log.Println("🔧 Initializing data provider...")
	if err := services.InitDataProvider(); err != nil {
		log.Fatalf("❌ Failed to initialize data provider: %v", err)
	}

	// Start OpenRouter Credit Monitor in background
	log.Println("🔍 Starting OpenRouter credit monitor...")
	go services.MonitorCredits()

	// Start AI Worker in background with graceful shutdown support
	aiWorker, err := worker.NewAIWorker()
	if err != nil {
		log.Fatalf("❌ Failed to initialize AI Worker: %v", err)
	}
	go func() {
		log.Println("Starting AI Worker...")
		aiWorker.Start()
	}()

	// Setup Gin router
	router := gin.Default()

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, token") // Added token header for WhatsApp session

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Home page
	router.GET("/", handlers.HomePage)

	// Health check
	router.GET("/health", handlers.HealthCheck)

	// WhatsApp Gateway routes - All WA API requests go through this gateway with /wa prefix
	// Admin routes bypass subscription checks, other routes validate subscription
	wa := router.Group("/wa")
	{
		// Admin endpoints (bypass all validation)
		wa.Any("/admin", handlers.WhatsAppGateway)       // Handle exact /wa/admin
		wa.Any("/admin/*path", handlers.WhatsAppGateway) // Handle /wa/admin/...

		// Session endpoints (validate subscription + session limits)
		wa.Any("/session", handlers.WhatsAppGateway)       // Handle exact /wa/session
		wa.Any("/session/*path", handlers.WhatsAppGateway) // Handle /wa/session/...

		// Webhook endpoints (validate subscription)
		wa.Any("/webhook", handlers.WhatsAppGateway)       // Handle exact /wa/webhook
		wa.Any("/webhook/*path", handlers.WhatsAppGateway) // Handle /wa/webhook/...

		// Chat endpoints (validate subscription + message tracking)
		wa.Any("/chat", handlers.WhatsAppGateway)       // Handle exact /wa/chat
		wa.Any("/chat/*path", handlers.WhatsAppGateway) // Handle /wa/chat/...

		// User endpoints (validate subscription)
		wa.Any("/user", handlers.WhatsAppGateway)       // Handle exact /wa/user
		wa.Any("/user/*path", handlers.WhatsAppGateway) // Handle /wa/user/...

		// Group endpoints (validate subscription)
		wa.Any("/group", handlers.WhatsAppGateway)       // Handle exact /wa/group
		wa.Any("/group/*path", handlers.WhatsAppGateway) // Handle /wa/group/...

		// Newsletter endpoints (validate subscription)
		wa.Any("/newsletter", handlers.WhatsAppGateway)       // Handle exact /wa/newsletter
		wa.Any("/newsletter/*path", handlers.WhatsAppGateway) // Handle /wa/newsletter/...
	}

	// AI webhook route - receives WhatsApp messages from WA Service for AI bot processing
	// This is the new architecture: WA Service → /webhook/ai → AI Worker
	router.POST("/webhook/ai", handlers.HandleAIWebhook)

	// Legacy webhook routes DIHAPUS - tidak dipakai lagi di arsitektur AI bot
	// Semua event handling sekarang dilakukan via /webhook/ai
	// Note: Jika masih ada service lain yang kirim ke /webhook/ai, perlu diubah ke /webhook/ai

	// Public cron job endpoint (no authentication required)
	router.GET("/bulk/cron/process", handlers.BulkCampaignCronJob)

	// Bulk contact and campaign endpoints
	bulk := router.Group("/bulk")
	bulk.Use(middleware.JWTMiddleware()) // Use JWT authentication instead of session
	{
		// Contact management
		bulk.POST("/contact/sync", handlers.BulkContactSync)
		bulk.GET("/contact", handlers.BulkContactList)
		bulk.POST("/contact/add", handlers.AddContacts)
		bulk.DELETE("/contact/delete", handlers.BulkDeleteContacts)

		// Campaign management endpoints
		campaign := bulk.Group("/campaign")
		{
			campaign.POST("", handlers.CreateCampaign)
			campaign.GET("", handlers.GetCampaigns)
			campaign.GET("/:id", handlers.GetCampaign)
			campaign.PUT("/:id", handlers.UpdateCampaign)
			campaign.DELETE("/:id", handlers.DeleteCampaign)
		}

		// Bulk campaign execution endpoints
		bulk.POST("/campaign/execute", handlers.CreateBulkCampaign)
		bulk.GET("/campaigns", handlers.GetBulkCampaigns)
		bulk.GET("/campaigns/:id", handlers.GetBulkCampaign)
		bulk.DELETE("/campaigns/:id", handlers.DeleteBulkCampaign)
	}

	// Get port from environment or default to 8070
	port := os.Getenv("PORT")
	if port == "" {
		port = "8070"
	}

	// Setup HTTP server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("🚀 Server starting on port %s", port)
		log.Printf("📡 Gateway mode: %s", os.Getenv("GATEWAY_MODE"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("🛑 Shutting down server...")

	// Stop AI Worker first
	log.Println("🤖 Stopping AI Worker...")
	aiWorker.Stop()

	// Give a deadline for HTTP server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited gracefully")
}
