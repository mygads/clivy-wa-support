package main

import (
	"log"
	"os"

	"genfity-wa-support/database"
	"genfity-wa-support/handlers"
	"genfity-wa-support/middleware"
	"genfity-wa-support/worker"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize database
	database.InitDatabase()

	// Start AI Worker in background goroutine
	go func() {
		log.Println("Starting AI Worker...")
		aiWorker := worker.NewAIWorker()
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
	// Note: Jika masih ada service lain yang kirim ke /webhook/wa, perlu diubah ke /webhook/ai

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

	log.Printf("Server starting on port %s", port)
	log.Printf("Gateway mode: %s", os.Getenv("GATEWAY_MODE"))
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
