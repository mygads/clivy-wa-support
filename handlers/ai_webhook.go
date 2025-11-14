package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/models"
	"genfity-wa-support/services"

	"github.com/gin-gonic/gin"
)

// WebhookPayload struktur dari WA Service
type WebhookPayload struct {
	InstanceName string `json:"instanceName"`
	Event        struct {
		Info struct {
			ID        string    `json:"ID"`
			Sender    string    `json:"Sender"`
			Chat      string    `json:"Chat"`
			Type      string    `json:"Type"`
			PushName  string    `json:"PushName"`
			Timestamp time.Time `json:"Timestamp"`
			IsFromMe  bool      `json:"IsFromMe"`
		} `json:"Info"`
		Message struct {
			ExtendedTextMessage struct {
				Text string `json:"text"`
			} `json:"extendedTextMessage"`
			Conversation string `json:"conversation"`
		} `json:"Message"`
	} `json:"event"`
}

// cleanJID removes device suffix from WhatsApp JID
// Example: "6281233784490:24@s.whatsapp.net" → "6281233784490@s.whatsapp.net"
func cleanJID(jid string) string {
	// Remove device suffix (:24, :26, etc)
	if strings.Contains(jid, ":") {
		parts := strings.Split(jid, ":")
		if len(parts) >= 2 {
			// Get phone number part and domain part
			phonePart := parts[0]
			domainPart := parts[len(parts)-1] // Get last part (e.g., "24@s.whatsapp.net")

			// Extract domain (everything after @)
			if strings.Contains(domainPart, "@") {
				domain := domainPart[strings.Index(domainPart, "@"):]
				return phonePart + domain
			}
		}
	}
	return jid
}

// HandleAIWebhook processes incoming WhatsApp messages for AI bot
func HandleAIWebhook(c *gin.Context) {
	var payload WebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("Invalid webhook payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// 1. Extract message data
	sessionToken := payload.InstanceName
	messageID := payload.Event.Info.ID
	from := cleanJID(payload.Event.Info.Sender) // Clean device suffix
	to := payload.Event.Info.Chat
	msgType := payload.Event.Info.Type
	pushName := payload.Event.Info.PushName
	timestamp := payload.Event.Info.Timestamp
	fromMe := payload.Event.Info.IsFromMe

	log.Printf("📨 Webhook received: session=%s, from=%s, type=%s, fromMe=%v",
		sessionToken, from, msgType, fromMe)

	// Skip pesan dari diri sendiri
	if fromMe {
		c.JSON(http.StatusOK, gin.H{"message": "Skipped: own message"})
		return
	}

	// Get message text
	var body string
	if payload.Event.Message.ExtendedTextMessage.Text != "" {
		body = payload.Event.Message.ExtendedTextMessage.Text
	} else {
		body = payload.Event.Message.Conversation
	}

	// Only process text messages for now
	if msgType != "text" || strings.TrimSpace(body) == "" {
		log.Printf("Non-text message ignored: type=%s", msgType)
		c.JSON(http.StatusOK, gin.H{"message": "Non-text message ignored"})
		return
	}

	log.Printf("💬 Message content: %s", body)

	// 2. Resolve session → user (call Transactional API)
	sessionInfo, err := services.ResolveSession(sessionToken)
	if err != nil {
		log.Printf("Failed to resolve session %s: %v", sessionToken, err)
		c.JSON(http.StatusOK, gin.H{"message": "Session not found", "error": err.Error()})
		return
	}

	log.Printf("✓ Session resolved: userID=%s, botActive=%v, subscriptionActive=%v",
		sessionInfo.UserID, sessionInfo.BotActive, sessionInfo.SubscriptionActive)

	// 3. Guard: Bot active & subscription active
	if !sessionInfo.BotActive {
		log.Printf("Bot inactive for session %s", sessionToken)
		c.JSON(http.StatusOK, gin.H{"message": "Bot inactive"})
		return
	}

	if !sessionInfo.SubscriptionActive {
		log.Printf("Subscription inactive for session %s", sessionToken)
		c.JSON(http.StatusOK, gin.H{"message": "Subscription inactive"})
		return
	}

	// 4. Save incoming message (idempotency via unique messageID)
	// Also triggers auto-cleanup (keep last 20 messages per contact)
	phoneNumber := strings.Split(from, "@")[0] // Extract phone number without @s.whatsapp.net
	if err := services.SaveIncomingMessageToAIChat(sessionToken, messageID, from, to, body, pushName, timestamp); err != nil {
		// Check if duplicate
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE constraint") {
			log.Printf("Duplicate message %s - skipped", messageID)
			c.JSON(http.StatusOK, gin.H{"message": "Duplicate message"})
			return
		}
		log.Printf("Failed to save chat message: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save message"})
		return
	}

	log.Printf("✓ Message saved to ai_chat_messages (contact: %s)", phoneNumber)

	// 4b. Save to permanent chat history (ChatRoom + ChatMessage)
	go func() {
		if err := services.SaveToChatHistory(sessionToken, from, to, body, pushName, timestamp, false); err != nil {
			log.Printf("⚠️  Failed to save to chat history: %v", err)
		}
	}()

	// 5. Enqueue AI job
	db := database.GetDB()
	aiJob := models.AIJob{
		Status:     "pending",
		Priority:   5,
		SessionTok: sessionToken,
		MessageID:  messageID,
		UserID:     sessionInfo.UserID,
		InputJSON:  body,
		Attempts:   0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := db.Create(&aiJob).Error; err != nil {
		log.Printf("Failed to enqueue AI job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue job"})
		return
	}

	// NOTIFY trigger will fire automatically via PostgreSQL trigger
	log.Printf("✅ Job #%d queued for AI processing (message: %s)", aiJob.ID, messageID)

	c.JSON(http.StatusOK, gin.H{
		"status":     "queued",
		"message_id": messageID,
		"job_id":     aiJob.ID,
	})
}
