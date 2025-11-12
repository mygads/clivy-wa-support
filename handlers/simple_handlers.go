package handlers

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// HomePage endpoint for root path
func HomePage(c *gin.Context) {
	now := time.Now()
	serverName := os.Getenv("SERVER_NAME")
	if serverName == "" {
		serverName = "Clivy WhatsApp Support API"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "running",
		"server":      serverName,
		"service":     "clivy-wa-support",
		"version":     "2.0.0-ai", // Versi baru dengan AI bot
		"mode":        "ai-bot",
		"time":        now.Format("2006-01-02 15:04:05"),
		"timezone":    now.Format("MST"),
		"timestamp":   now.Unix(),
		"message":     "AI Bot Server is running successfully",
		"environment": gin.Mode(),
	})
}

// HealthCheck endpoint
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"time":    time.Now().Format(time.RFC3339),
		"service": "clivy-wa-support",
		"version": "2.0.0-ai",
		"mode":    "ai-bot",
	})
}
