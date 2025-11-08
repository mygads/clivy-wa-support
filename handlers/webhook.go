package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DirectWuzAPIWebhookData represents the actual structure sent by WuzAPI
type DirectWuzAPIWebhookData struct {
	Event map[string]interface{} `json:"event"`
	Type  string                 `json:"type"`
	Token string                 `json:"token"`
}

// Raw map for debugging
type WebhookRawData map[string]interface{}

// ClivyWebhookData represents the incoming webhook structure from GENFITY (legacy)
type ClivyWebhookData struct {
	Event     string                 `json:"event"`
	Data      map[string]interface{} `json:"data"`
	Timestamp string                 `json:"timestamp"`
	UserToken string                 `json:"user_token"`
}

// VerifyWebhook handles webhook verification (optional for GENFITY)
func VerifyWebhook(c *gin.Context) {
	// Check if verification is enabled
	verifyToken := os.Getenv("WEBHOOK_VERIFY_TOKEN")

	if verifyToken == "" {
		// No verification token set, skip verification
		c.JSON(http.StatusOK, gin.H{"message": "Webhook verification disabled"})
		return
	}

	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode == "subscribe" && token == verifyToken {
		c.String(http.StatusOK, challenge)
		return
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
}

// HandleWhatsAppWebhook processes incoming WhatsApp webhooks from WuzAPI
func HandleWhatsAppWebhook(c *gin.Context) {
	// Parse the direct webhook structure
	var wuzapiData DirectWuzAPIWebhookData
	if err := c.ShouldBindJSON(&wuzapiData); err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data"})
		return
	}

	log.Printf("DEBUG: Successfully parsed webhook data")
	log.Printf("DEBUG: Event type: %s", wuzapiData.Type)
	log.Printf("DEBUG: Token: %s", wuzapiData.Token)
	log.Printf("DEBUG: Event data: %+v", wuzapiData.Event)

	// Check user settings to see if chat log is enabled
	db := database.GetDB()
	var userSettings models.UserSettings
	if err := db.Where("user_token = ?", wuzapiData.Token).First(&userSettings).Error; err != nil {
		// Create default user settings with chat log disabled
		userSettings = models.UserSettings{
			UserToken:      wuzapiData.Token,
			ChatLogEnabled: false, // Default: disabled
			IsActive:       true,
		}
		db.Create(&userSettings)
		log.Printf("Created default user settings for token: %s (chat log disabled)", wuzapiData.Token)
	}

	// Convert to legacy format for compatibility with existing processing functions
	webhookData := ClivyWebhookData{
		Event:     wuzapiData.Type,
		Data:      wuzapiData.Event,
		UserToken: wuzapiData.Token,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Convert raw webhook data to JSON string for storage
	rawDataJSON, _ := json.Marshal(wuzapiData)

	// Store the raw webhook event first
	webhookEvent := models.GenEventWebhook{
		EventType: webhookData.Event,
		Source:    "wa",
		UserToken: webhookData.UserToken,
		EventData: models.JSONB(webhookData.Data),
		RawData:   string(rawDataJSON),
		Processed: false,
	}

	if err := db.Create(&webhookEvent).Error; err != nil {
		log.Printf("Error storing webhook event: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store webhook event"})
		return
	}

	log.Printf("DEBUG: Stored webhook event with ID: %d", webhookEvent.ID)

	// Process the event based on type and user settings
	go func() {
		if err := processWebhookEventWithSettings(webhookData, userSettings); err != nil {
			log.Printf("Error processing webhook event: %v", err)
			// Update the webhook event as failed to process
			db.Model(&webhookEvent).Update("processed", false)
		} else {
			// Mark as processed
			now := time.Now()
			db.Model(&webhookEvent).Updates(models.GenEventWebhook{
				Processed:   true,
				ProcessedAt: &now,
			})
			log.Printf("DEBUG: Successfully processed webhook event with ID: %d", webhookEvent.ID)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// processWebhookEventWithSettings processes different types of webhook events based on user settings
func processWebhookEventWithSettings(webhookData ClivyWebhookData, userSettings models.UserSettings) error {
	// Always process session-related events regardless of chat log setting
	switch webhookData.Event {
	case "Connected", "QR", "Disconnected":
		return processWebhookEvent(webhookData)
	}

	// For message events, check if chat log is enabled
	if !userSettings.ChatLogEnabled {
		log.Printf("Chat log disabled for user %s, skipping message storage", webhookData.UserToken)
		return nil // Skip processing but don't return error
	}

	// Process the event normally if chat log is enabled
	switch webhookData.Event {
	case "Message":
		return processMessageEventWithChatRoom(webhookData)
	case "MessageSent":
		return processMessageSentEventWithChatRoom(webhookData)
	case "ReadReceipt":
		return processReadReceiptEventWithChatRoom(webhookData)
	default:
		return processWebhookEvent(webhookData) // Fallback to original processing
	}
}

// processWebhookEvent processes different types of webhook events
func processWebhookEvent(webhookData ClivyWebhookData) error {
	switch webhookData.Event {
	case "Message":
		return processMessageEvent(webhookData)
	case "MessageSent":
		return processMessageSentEvent(webhookData)
	case "ReadReceipt":
		return processReadReceiptEvent(webhookData)
	case "Presence":
		return processPresenceEvent(webhookData)
	case "ChatPresence":
		return processChatPresenceEvent(webhookData)
	case "HistorySync":
		return processHistorySyncEvent(webhookData)
	case "Connected":
		return processConnectedEvent(webhookData)
	case "QR":
		return processQREvent(webhookData)
	default:
		log.Printf("Unknown event type: %s", webhookData.Event)
		return nil
	}
}

// processMessageEvent processes message events
func processMessageEvent(webhookData ClivyWebhookData) error {
	data := webhookData.Data

	// Extract message info from the new format structure
	var messageInfo map[string]interface{}
	var messageData map[string]interface{}

	if info, ok := data["Info"].(map[string]interface{}); ok {
		messageInfo = info
	} else {
		// Fallback to old format
		messageInfo = data
	}

	if message, ok := data["Message"].(map[string]interface{}); ok {
		messageData = message
	} else {
		// Fallback to old format
		messageData = data
	}

	// Extract basic message info from Info section
	messageID, _ := messageInfo["ID"].(string)
	if messageID == "" {
		messageID, _ = data["MessageID"].(string)
		if messageID == "" {
			messageID, _ = data["id"].(string)
		}
	}

	from, _ := messageInfo["Sender"].(string)
	if from == "" {
		from, _ = data["from"].(string)
	}

	to, _ := messageInfo["Chat"].(string)
	if to == "" {
		to, _ = data["to"].(string)
	}

	// Clean sender format - remove device suffix (:24) from sender
	if strings.Contains(from, ":") {
		parts := strings.Split(from, ":")
		if len(parts) > 0 {
			from = parts[0] + "@s.whatsapp.net"
		}
	}

	fromMe, _ := messageInfo["IsFromMe"].(bool)
	if !fromMe {
		fromMe, _ = data["fromMe"].(bool)
	}

	// For incoming messages (IsFromMe=false), we need to determine the correct recipient
	// The Chat field usually contains the conversation ID
	// For person-to-person chat, Chat field should be the recipient (our session owner)
	if !fromMe {
		// Message from external user to our session owner
		// We can get the session owner's JID from the sessions table
		db := database.GetDB()
		var session models.WhatsAppSession
		if err := db.Where("user_token = ?", webhookData.UserToken).First(&session).Error; err == nil {
			if session.JID != "" {
				// Use the session's JID as the recipient
				to = session.JID
			}
		}
	}

	pushName, _ := messageInfo["PushName"].(string)
	if pushName == "" {
		pushName, _ = data["pushname"].(string)
	}

	messageType, _ := messageInfo["Type"].(string)
	if messageType == "" {
		messageType, _ = data["type"].(string)
	}

	timestampStr, _ := messageInfo["Timestamp"].(string)
	if timestampStr == "" {
		timestampStr, _ = data["timestamp"].(string)
	}

	isGroup, _ := messageInfo["IsGroup"].(bool)

	// Parse timestamp
	messageTimestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		// Try parsing as Unix timestamp if RFC3339 fails
		if timestampInt, ok := messageInfo["Timestamp"].(float64); ok {
			messageTimestamp = time.Unix(int64(timestampInt), 0)
		} else if timestampInt, ok := data["timestamp"].(float64); ok {
			messageTimestamp = time.Unix(int64(timestampInt), 0)
		} else {
			log.Printf("Error parsing timestamp: %v", err)
			messageTimestamp = time.Now()
		}
	}

	// Apply filtering
	if shouldFilterMessage(messageType, messageData, from) {
		log.Printf("Message filtered: %s from %s", messageType, from)
		return nil
	}

	// Create WhatsApp message record
	message := models.WhatsAppMessage{
		MessageID:        messageID,
		From:             from,
		To:               to,
		FromMe:           fromMe,
		PushName:         pushName,
		MessageType:      messageType,
		MessageTimestamp: messageTimestamp,
		UserToken:        webhookData.UserToken,
		Status:           "received",
		Processed:        false,
	}

	// Set group info if it's a group message
	if isGroup {
		message.GroupJid = to
		if participant, ok := messageInfo["Participant"].(string); ok {
			message.Participant = participant
		}
	}

	// Extract content based on message type
	var body string
	switch messageType {
	case "text":
		// Handle both conversation and extendedTextMessage
		if conversation, ok := messageData["conversation"].(string); ok {
			body = conversation
		} else if extendedText, ok := messageData["extendedTextMessage"].(map[string]interface{}); ok {
			if text, ok := extendedText["text"].(string); ok {
				body = text
			}
		} else if bodyText, ok := messageData["body"].(string); ok {
			body = bodyText
		}
		message.Body = body

	case "image", "video", "audio", "document", "sticker":
		if bodyText, ok := messageData["body"].(string); ok {
			message.Body = bodyText
		}
		if caption, ok := messageData["caption"].(string); ok {
			message.Caption = caption
		}
		// Handle media from different sources
		if media, ok := messageData["media"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(media)
		} else if imageMessage, ok := messageData["imageMessage"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(imageMessage)
		} else if videoMessage, ok := messageData["videoMessage"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(videoMessage)
		} else if audioMessage, ok := messageData["audioMessage"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(audioMessage)
		} else if documentMessage, ok := messageData["documentMessage"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(documentMessage)
		} else if stickerMessage, ok := messageData["stickerMessage"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(stickerMessage)
		}

	case "location":
		if location, ok := messageData["location"].(map[string]interface{}); ok {
			message.LocationData = models.JSONB(location)
		} else if locationMessage, ok := messageData["locationMessage"].(map[string]interface{}); ok {
			message.LocationData = models.JSONB(locationMessage)
		}

	case "contact":
		if contact, ok := messageData["contact"].(map[string]interface{}); ok {
			message.ContactData = models.JSONB(contact)
		} else if contactMessage, ok := messageData["contactMessage"].(map[string]interface{}); ok {
			message.ContactData = models.JSONB(contactMessage)
		}
	}

	// Handle quoted/replied messages
	if quotedMessage, ok := messageData["quotedMessage"].(map[string]interface{}); ok {
		message.QuotedMessage = models.JSONB(quotedMessage)
	}

	// Handle context info for extended text messages
	if extendedText, ok := messageData["extendedTextMessage"].(map[string]interface{}); ok {
		if contextInfo, ok := extendedText["contextInfo"].(map[string]interface{}); ok {
			message.QuotedMessage = models.JSONB(contextInfo)
		}
	}

	// Handle mentions
	if mentionedJid, ok := messageData["mentionedJid"].([]interface{}); ok {
		message.MentionedJid = models.JSONB{"jids": mentionedJid}
	}

	// Handle forwarded flag
	if data["Forwarded"] != nil {
		if forwarded, ok := data["Forwarded"].(bool); ok {
			message.Forwarded = forwarded
		}
	}

	// Handle broadcast flag
	if data["Broadcast"] != nil {
		if broadcast, ok := data["Broadcast"].(bool); ok {
			message.Broadcast = broadcast
		}
	}

	// Handle ephemeral duration
	if ephemeralDuration, ok := data["EphemeralDuration"].(float64); ok {
		message.EphemeralDuration = int(ephemeralDuration)
	}

	db := database.GetDB()
	return db.Create(&message).Error
}

// processMessageSentEvent processes message sent events (outgoing messages from our user)
func processMessageSentEvent(webhookData ClivyWebhookData) error {
	data := webhookData.Data

	// Extract message info from the new format structure
	var messageInfo map[string]interface{}
	var messageData map[string]interface{}

	if info, ok := data["Info"].(map[string]interface{}); ok {
		messageInfo = info
	} else {
		// Fallback to old format
		messageInfo = data
	}

	if message, ok := data["Message"].(map[string]interface{}); ok {
		messageData = message
	} else {
		// Fallback to old format
		messageData = data
	}

	// Extract basic message info from Info section
	messageID, _ := messageInfo["ID"].(string)
	if messageID == "" {
		messageID, _ = data["MessageID"].(string)
		if messageID == "" {
			messageID, _ = data["id"].(string)
		}
	}

	// For MessageSent, IsFromMe should be true (we sent the message)
	fromMe := true
	if isFromMe, ok := messageInfo["IsFromMe"].(bool); ok {
		fromMe = isFromMe
	}

	// Get session owner's JID as sender (since we sent the message)
	db := database.GetDB()
	var session models.WhatsAppSession
	var senderJID string
	if err := db.Where("user_token = ?", webhookData.UserToken).First(&session).Error; err == nil {
		if session.JID != "" {
			senderJID = session.JID
		}
	}

	// Chat field contains the recipient
	to, _ := messageInfo["Chat"].(string)
	if to == "" {
		to, _ = data["to"].(string)
	}

	from := senderJID // We are the sender
	if from == "" {
		// Fallback if session JID not available
		from = webhookData.UserToken + "@s.whatsapp.net"
	}

	messageType, _ := messageInfo["MessageType"].(string)
	if messageType == "" {
		messageType, _ = messageInfo["Type"].(string)
		if messageType == "" {
			messageType, _ = data["type"].(string)
		}
	}

	timestampStr, _ := messageInfo["Timestamp"].(string)
	if timestampStr == "" {
		timestampStr, _ = data["timestamp"].(string)
	}

	// Parse timestamp
	messageTimestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		// Try parsing as Unix timestamp if RFC3339 fails
		if timestampInt, ok := messageInfo["Timestamp"].(float64); ok {
			messageTimestamp = time.Unix(int64(timestampInt), 0)
		} else if timestampInt, ok := data["timestamp"].(float64); ok {
			messageTimestamp = time.Unix(int64(timestampInt), 0)
		} else {
			log.Printf("Error parsing timestamp: %v", err)
			messageTimestamp = time.Now()
		}
	}

	// Create WhatsApp message record
	message := models.WhatsAppMessage{
		MessageID:        messageID,
		From:             from,
		To:               to,
		FromMe:           fromMe,
		PushName:         "", // Not applicable for sent messages
		MessageType:      messageType,
		MessageTimestamp: messageTimestamp,
		UserToken:        webhookData.UserToken,
		Status:           "sent",
		Processed:        false,
	}

	// Extract content based on message type
	var body string
	switch messageType {
	case "text":
		// Handle both conversation and extendedTextMessage
		if conversation, ok := messageData["conversation"].(string); ok {
			body = conversation
		} else if extendedText, ok := messageData["extendedTextMessage"].(map[string]interface{}); ok {
			if text, ok := extendedText["text"].(string); ok {
				body = text
			}
		} else if bodyText, ok := messageData["body"].(string); ok {
			body = bodyText
		}
		message.Body = body

	case "image", "video", "audio", "document", "sticker":
		if bodyText, ok := messageData["body"].(string); ok {
			message.Body = bodyText
		}
		if caption, ok := messageData["caption"].(string); ok {
			message.Caption = caption
		}
		// Handle media from different sources
		if media, ok := messageData["media"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(media)
		} else if imageMessage, ok := messageData["imageMessage"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(imageMessage)
		} else if videoMessage, ok := messageData["videoMessage"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(videoMessage)
		} else if audioMessage, ok := messageData["audioMessage"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(audioMessage)
		} else if documentMessage, ok := messageData["documentMessage"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(documentMessage)
		} else if stickerMessage, ok := messageData["stickerMessage"].(map[string]interface{}); ok {
			message.MediaData = models.JSONB(stickerMessage)
		}

	case "location":
		if location, ok := messageData["location"].(map[string]interface{}); ok {
			message.LocationData = models.JSONB(location)
		} else if locationMessage, ok := messageData["locationMessage"].(map[string]interface{}); ok {
			message.LocationData = models.JSONB(locationMessage)
		}

	case "contact":
		if contact, ok := messageData["contact"].(map[string]interface{}); ok {
			message.ContactData = models.JSONB(contact)
		} else if contactMessage, ok := messageData["contactMessage"].(map[string]interface{}); ok {
			message.ContactData = models.JSONB(contactMessage)
		}
	}

	// Handle quoted/replied messages
	if quotedMessage, ok := messageData["quotedMessage"].(map[string]interface{}); ok {
		message.QuotedMessage = models.JSONB(quotedMessage)
	}

	// Handle context info for extended text messages
	if extendedText, ok := messageData["extendedTextMessage"].(map[string]interface{}); ok {
		if contextInfo, ok := extendedText["contextInfo"].(map[string]interface{}); ok {
			message.QuotedMessage = models.JSONB(contextInfo)
		}
	}

	// Handle mentions
	if mentionedJid, ok := messageData["mentionedJid"].([]interface{}); ok {
		message.MentionedJid = models.JSONB{"jids": mentionedJid}
	}

	return db.Create(&message).Error
}

// processReadReceiptEvent processes read receipt events
func processReadReceiptEvent(webhookData ClivyWebhookData) error {
	data := webhookData.Data

	// Extract info from new format structure
	var eventInfo map[string]interface{}

	if event, ok := data["event"].(map[string]interface{}); ok {
		eventInfo = event
	} else {
		eventInfo = data
	}

	sender, _ := eventInfo["Sender"].(string)
	if sender == "" {
		sender, _ = data["from"].(string)
	}

	// Clean sender format - remove device suffix (:24, :26) from sender
	if strings.Contains(sender, ":") {
		parts := strings.Split(sender, ":")
		if len(parts) > 0 {
			sender = parts[0] + "@s.whatsapp.net"
		}
	}

	chatJid, _ := eventInfo["Chat"].(string)
	if chatJid == "" {
		chatJid, _ = data["to"].(string)
	}

	// Get status from the new format
	status, _ := data["state"].(string)
	if status == "" {
		status, _ := eventInfo["Type"].(string)
		if status == "read" {
			status = "Read"
		}
	}

	// Convert status to lowercase for consistency
	receiptType := strings.ToLower(status)

	timestampStr, _ := eventInfo["Timestamp"].(string)
	if timestampStr == "" {
		timestampStr, _ = data["timestamp"].(string)
	}

	// Parse timestamp
	eventTimestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		eventTimestamp = time.Now()
	}

	// For ReadReceipt, the logic should be:
	// - sender = who performed the read/delivered action
	// - chatJid = the conversation/chat ID
	// - For person-to-person chat, we need to determine who is the message owner

	var messageOwner string
	db := database.GetDB()

	// Get session owner's JID to determine message flow
	var session models.WhatsAppSession
	if err := db.Where("user_token = ?", webhookData.UserToken).First(&session).Error; err == nil {
		if session.JID != "" {
			messageOwner = session.JID
		}
	}

	// Determine from and to based on who read whose message
	var from, to string

	if sender == messageOwner {
		// Our session owner read a message - message was sent TO us by chatJid person
		from = chatJid // Original message sender
		to = sender    // Our session owner (who read it)
	} else {
		// External person read our message - message was sent BY us to external person
		from = messageOwner // Our session owner (original message sender)
		to = sender         // External person (who read it)
	}

	// Extract message IDs from new format
	var messageIds models.JSONB
	if msgIds, ok := eventInfo["MessageIDs"].([]interface{}); ok {
		messageIds = models.JSONB{"ids": msgIds}

		// Check for duplicates and update message status
		go updateMessageStatusWithDuplicateCheck(msgIds, receiptType, eventTimestamp, webhookData.UserToken)
	} else if msgIds, ok := data["messageIds"].([]interface{}); ok {
		messageIds = models.JSONB{"ids": msgIds}

		// Check for duplicates and update message status
		go updateMessageStatusWithDuplicateCheck(msgIds, receiptType, eventTimestamp, webhookData.UserToken)
	}

	// Check if this exact receipt already exists to prevent duplicates
	var existingCount int64
	db.Model(&models.WhatsAppReadReceipt{}).
		Where("message_ids = ? AND receipt_type = ? AND user_token = ?",
			messageIds, receiptType, webhookData.UserToken).
		Count(&existingCount)

	if existingCount > 0 {
		log.Printf("Duplicate ReadReceipt ignored: %s %s", receiptType, messageIds)
		return nil // Skip duplicate
	}

	receipt := models.WhatsAppReadReceipt{
		MessageIds:     messageIds,
		From:           from,
		To:             to,
		ReceiptType:    receiptType,
		EventTimestamp: eventTimestamp,
		UserToken:      webhookData.UserToken,
	}

	return db.Create(&receipt).Error
}

// processPresenceEvent processes presence events
func processPresenceEvent(webhookData ClivyWebhookData) error {
	data := webhookData.Data

	// Handle both WuzAPI and legacy field names
	from, _ := data["Sender"].(string)
	if from == "" {
		from, _ = data["from"].(string)
	}

	presence, _ := data["State"].(string)
	if presence == "" {
		presence, _ = data["presence"].(string)
	}

	lastSeenStr, _ := data["LastSeen"].(string)
	if lastSeenStr == "" {
		lastSeenStr, _ = data["lastSeen"].(string)
	}

	var lastSeen *time.Time
	if lastSeenStr != "" {
		if parsed, err := time.Parse(time.RFC3339, lastSeenStr); err == nil {
			lastSeen = &parsed
		}
	}

	presenceEvent := models.WhatsAppPresence{
		From:      from,
		Presence:  presence,
		LastSeen:  lastSeen,
		UserToken: webhookData.UserToken,
	}

	db := database.GetDB()
	return db.Create(&presenceEvent).Error
}

// processChatPresenceEvent processes chat presence events
func processChatPresenceEvent(webhookData ClivyWebhookData) error {
	data := webhookData.Data

	// Extract from new format structure
	var eventInfo map[string]interface{}

	if event, ok := data["event"].(map[string]interface{}); ok {
		eventInfo = event
	} else {
		eventInfo = data
	}

	from, _ := eventInfo["Sender"].(string)
	if from == "" {
		from, _ = data["from"].(string)
	}

	chatJid, _ := eventInfo["Chat"].(string)
	if chatJid == "" {
		chatJid, _ = data["chatJid"].(string)
	}

	state, _ := eventInfo["State"].(string)
	if state == "" {
		state, _ = data["state"].(string)
	}

	media, _ := eventInfo["Media"].(string)
	if media == "" {
		media, _ = data["media"].(string)
	}

	db := database.GetDB()

	// Find existing chat presence record for this from+chat_jid combination
	var chatPresence models.WhatsAppChatPresence
	result := db.Where(`"from" = ? AND chat_jid = ? AND user_token = ?`, from, chatJid, webhookData.UserToken).First(&chatPresence)

	if result.Error != nil {
		// Create new record
		chatPresence = models.WhatsAppChatPresence{
			From:      from,
			ChatJid:   chatJid,
			State:     state,
			Media:     media,
			UserToken: webhookData.UserToken,
		}
	} else {
		// Update existing record
		chatPresence.State = state
		chatPresence.Media = media
		chatPresence.AutoStopped = false // Reset auto-stopped flag
		chatPresence.ExpiresAt = nil     // Reset expiration
	}

	// If state is composing, set expiration time for auto-stop
	if state == "composing" {
		expirationTime := time.Now().Add(10 * time.Second)
		chatPresence.ExpiresAt = &expirationTime

		// Schedule auto-stop for typing after 10 seconds
		go autoStopTyping(from, chatJid, webhookData.UserToken, expirationTime)
	}

	// Save (create or update)
	if result.Error != nil {
		return db.Create(&chatPresence).Error
	} else {
		return db.Save(&chatPresence).Error
	}
}

// processHistorySyncEvent processes history sync events
func processHistorySyncEvent(webhookData ClivyWebhookData) error {
	data := webhookData.Data

	syncType, _ := data["syncType"].(string)
	conversations, _ := data["conversations"].([]interface{})

	historySync := models.WhatsAppHistorySync{
		SyncType:      syncType,
		Conversations: models.JSONB{"conversations": conversations},
		UserToken:     webhookData.UserToken,
	}

	db := database.GetDB()
	return db.Create(&historySync).Error
}

// shouldFilterMessage applies filtering logic to messages
func shouldFilterMessage(messageType string, data map[string]interface{}, from string) bool {
	// Add your filtering logic here

	// Filter out empty text messages
	if messageType == "text" {
		if body, ok := data["body"].(string); ok && strings.TrimSpace(body) == "" {
			return true
		}
		if conversation, ok := data["conversation"].(string); ok && strings.TrimSpace(conversation) == "" {
			return true
		}
	}

	// Example: Filter out specific phone numbers
	// if from == "1234567890@s.whatsapp.net" {
	//     return true
	// }

	// Example: Filter out messages with specific keywords
	// if body, ok := data["body"].(string); ok {
	//     if strings.Contains(strings.ToLower(body), "spam") {
	//         return true
	//     }
	// }

	return false
}

// updateMessageStatusWithDuplicateCheck updates message status with duplicate prevention
func updateMessageStatusWithDuplicateCheck(messageIds []interface{}, status string, timestamp time.Time, userToken string) {
	db := database.GetDB()

	for _, msgId := range messageIds {
		if messageID, ok := msgId.(string); ok {
			// Check if this exact status already exists for this message
			var existingCount int64
			db.Model(&models.WhatsAppMessageStatus{}).
				Where("message_id = ? AND status = ? AND user_token = ?", messageID, status, userToken).
				Count(&existingCount)

			if existingCount > 0 {
				log.Printf("Duplicate message status ignored: %s %s", messageID, status)
				continue // Skip duplicate
			}

			// Create message status record
			messageStatus := models.WhatsAppMessageStatus{
				MessageID:      messageID,
				Status:         status,
				EventTimestamp: timestamp,
				UserToken:      userToken,
			}

			// Insert status record
			if err := db.Create(&messageStatus).Error; err != nil {
				log.Printf("Error creating message status: %v", err)
				continue
			}

			// Also update the main message record status (only if it's a progression)
			var statusToUpdate string
			switch status {
			case "delivered":
				statusToUpdate = "delivered"
			case "read":
				statusToUpdate = "read"
			default:
				statusToUpdate = status
			}

			// Only update if the new status is a progression (sent -> delivered -> read)
			statusPriority := map[string]int{
				"sent":      1,
				"delivered": 2,
				"read":      3,
			}

			var currentMessage models.WhatsAppMessage
			if err := db.Where("message_id = ? AND user_token = ?", messageID, userToken).First(&currentMessage).Error; err == nil {
				currentPriority := statusPriority[currentMessage.Status]
				newPriority := statusPriority[statusToUpdate]

				if newPriority > currentPriority {
					db.Model(&models.WhatsAppMessage{}).
						Where("message_id = ? AND user_token = ?", messageID, userToken).
						Update("status", statusToUpdate)
				}
			}
		}
	}
}

// updateMessageStatus updates the status of messages in the database (legacy function)
func updateMessageStatus(messageIds []interface{}, status string, timestamp time.Time, userToken string) {
	// Use the new duplicate check function
	updateMessageStatusWithDuplicateCheck(messageIds, status, timestamp, userToken)
}

// autoStopTyping automatically sets typing status to paused after 10 seconds
func autoStopTyping(from, chatJid, userToken string, expirationTime time.Time) {
	time.Sleep(10 * time.Second)

	db := database.GetDB()

	// Check if there's still a composing status that hasn't been updated
	var count int64
	db.Model(&models.WhatsAppChatPresence{}).
		Where(`"from" = ? AND chat_jid = ? AND user_token = ? AND state = ? AND expires_at = ?`,
			from, chatJid, userToken, "composing", expirationTime).
		Count(&count)

	if count > 0 {
		// Update to paused and mark as auto-stopped
		db.Model(&models.WhatsAppChatPresence{}).
			Where(`"from" = ? AND chat_jid = ? AND user_token = ? AND state = ? AND expires_at = ?`,
				from, chatJid, userToken, "composing", expirationTime).
			Updates(map[string]interface{}{
				"state":        "paused",
				"auto_stopped": true,
			})

		log.Printf("Auto-stopped typing for %s in chat %s", from, chatJid)
	}
}

// processConnectedEvent processes connection events
func processConnectedEvent(webhookData ClivyWebhookData) error {
	data := webhookData.Data

	db := database.GetDB()

	// Find existing session record
	var session models.WhatsAppSession
	result := db.Where("user_token = ?", webhookData.UserToken).First(&session)

	if result.Error != nil {
		// Create new session record
		session = models.WhatsAppSession{
			UserToken:    webhookData.UserToken,
			SessionState: "connected",
			Connected:    true,
			LoggedIn:     true,
		}
	} else {
		// Update existing session
		session.SessionState = "connected"
		session.Connected = true
		session.LoggedIn = true
	}

	// Set connection time
	now := time.Now()
	session.ConnectedAt = &now
	session.DisconnectedAt = nil
	session.QRCode = "" // Clear QR code when connected
	session.QRExpiredAt = nil

	// Check if event has additional connection info
	if eventData, ok := data["event"].(map[string]interface{}); ok {
		if action, ok := eventData["Action"].(map[string]interface{}); ok {
			if name, ok := action["name"].(string); ok && name == "~" {
				// Full connection established
				session.SessionState = "connected"
			}
		}
	}

	if result.Error != nil {
		return db.Create(&session).Error
	} else {
		return db.Save(&session).Error
	}
}

// processQREvent processes QR code events
func processQREvent(webhookData ClivyWebhookData) error {
	data := webhookData.Data

	qrCode, _ := data["qrCodeBase64"].(string)
	if qrCode == "" {
		qrCode, _ = data["event"].(string)
	}

	db := database.GetDB()

	// Find existing session record
	var session models.WhatsAppSession
	result := db.Where("user_token = ?", webhookData.UserToken).First(&session)

	if result.Error != nil {
		// Create new session record
		session = models.WhatsAppSession{
			UserToken:    webhookData.UserToken,
			SessionState: "qr_waiting",
			Connected:    false,
			LoggedIn:     false,
		}
	} else {
		// Update existing session
		session.SessionState = "qr_waiting"
		session.Connected = false
		session.LoggedIn = false
		session.ConnectedAt = nil
		session.DisconnectedAt = nil
	}

	// Set QR code and expiration (QR codes typically expire after 60 seconds)
	session.QRCode = qrCode
	expirationTime := time.Now().Add(60 * time.Second)
	session.QRExpiredAt = &expirationTime

	if result.Error != nil {
		return db.Create(&session).Error
	} else {
		return db.Save(&session).Error
	}
}

// GetMessages retrieves stored messages with pagination and user_token filtering
func GetMessages(c *gin.Context) {
	db := database.GetDB()

	var messages []models.WhatsAppMessage

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	userToken := c.Query("user_token")     // Filter by specific user/WA number
	messageType := c.Query("message_type") // Filter by message type

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Build query with filters
	query := db.Model(&models.WhatsAppMessage{})

	if userToken != "" {
		query = query.Where("user_token = ?", userToken)
	}

	if messageType != "" {
		query = query.Where("message_type = ?", messageType)
	}

	var count int64
	query.Count(&count)

	if err := query.Limit(limit).Offset(offset).Order("message_timestamp desc").Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"total":    count,
		"page":     page,
		"limit":    limit,
		"pages":    (count + int64(limit) - 1) / int64(limit),
		"filters": map[string]interface{}{
			"user_token":   userToken,
			"message_type": messageType,
		},
	})
}

// GetWebhookEvents retrieves webhook events with pagination and filtering
func GetWebhookEvents(c *gin.Context) {
	db := database.GetDB()

	var events []models.GenEventWebhook

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	eventType := c.Query("event_type")
	source := c.DefaultQuery("source", "wa")
	userToken := c.Query("user_token") // Filter by specific user/WA number

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	query := db.Model(&models.GenEventWebhook{}).Where("source = ?", source)

	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	if userToken != "" {
		query = query.Where("user_token = ?", userToken)
	}

	var count int64
	query.Count(&count)

	if err := query.Limit(limit).Offset(offset).Order("received_at desc").Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve webhook events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  count,
		"page":   page,
		"limit":  limit,
		"pages":  (count + int64(limit) - 1) / int64(limit),
		"filters": map[string]interface{}{
			"event_type": eventType,
			"source":     source,
			"user_token": userToken,
		},
	})
}

// GetUsers retrieves active user_tokens from the database
func GetUsers(c *gin.Context) {
	db := database.GetDB()

	// Get unique user_tokens from webhook events
	var userTokens []struct {
		UserToken    string    `json:"user_token"`
		MessageCount int64     `json:"message_count"`
		LastActivity time.Time `json:"last_activity"`
		TotalEvents  int64     `json:"total_events"`
	}

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Get user tokens with statistics
	err := db.Table("gen_event_webhooks").
		Select(`
			user_token,
			COUNT(CASE WHEN event_type = 'Message' THEN 1 END) as message_count,
			MAX(received_at) as last_activity,
			COUNT(*) as total_events
		`).
		Where("user_token != ''").
		Group("user_token").
		Order("last_activity desc").
		Limit(limit).
		Offset(offset).
		Find(&userTokens).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user tokens"})
		return
	}

	// Get total count of unique user tokens
	var totalCount int64
	db.Table("gen_event_webhooks").
		Select("DISTINCT user_token").
		Where("user_token != ''").
		Count(&totalCount)

	c.JSON(http.StatusOK, gin.H{
		"users": userTokens,
		"total": totalCount,
		"page":  page,
		"limit": limit,
		"pages": (totalCount + int64(limit) - 1) / int64(limit),
	})
}

// GetUserStats retrieves statistics for a specific user_token
func GetUserStats(c *gin.Context) {
	userToken := c.Param("user_token")
	if userToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_token is required"})
		return
	}

	db := database.GetDB()

	// Check if user_token exists in our database
	var eventCount int64
	db.Model(&models.GenEventWebhook{}).Where("user_token = ?", userToken).Count(&eventCount)

	if eventCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User token not found"})
		return
	}

	// Get message counts by type
	var messageStats []struct {
		MessageType string `json:"message_type"`
		Count       int64  `json:"count"`
	}

	db.Model(&models.WhatsAppMessage{}).
		Select("message_type, count(*) as count").
		Where("user_token = ?", userToken).
		Group("message_type").
		Find(&messageStats)

	// Get total counts
	var totalMessages int64
	var totalEvents int64

	db.Model(&models.WhatsAppMessage{}).Where("user_token = ?", userToken).Count(&totalMessages)
	db.Model(&models.GenEventWebhook{}).Where("user_token = ?", userToken).Count(&totalEvents)

	// Get recent activity (last 24 hours)
	yesterday := time.Now().AddDate(0, 0, -1)
	var recentMessages int64
	var recentEvents int64

	db.Model(&models.WhatsAppMessage{}).
		Where("user_token = ? AND message_timestamp >= ?", userToken, yesterday).
		Count(&recentMessages)

	db.Model(&models.GenEventWebhook{}).
		Where("user_token = ? AND received_at >= ?", userToken, yesterday).
		Count(&recentEvents)

	// Get first and last activity
	var firstActivity time.Time
	var lastActivity time.Time

	db.Model(&models.GenEventWebhook{}).
		Select("MIN(received_at)").
		Where("user_token = ?", userToken).
		Scan(&firstActivity)

	db.Model(&models.GenEventWebhook{}).
		Select("MAX(received_at)").
		Where("user_token = ?", userToken).
		Scan(&lastActivity)

	c.JSON(http.StatusOK, gin.H{
		"user_token": userToken,
		"stats": map[string]interface{}{
			"total_messages":      totalMessages,
			"total_events":        totalEvents,
			"recent_messages_24h": recentMessages,
			"recent_events_24h":   recentEvents,
			"message_types":       messageStats,
			"first_activity":      firstActivity,
			"last_activity":       lastActivity,
		},
	})
}

// HealthCheck endpoint
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"time":    time.Now().Format(time.RFC3339),
		"service": "clivy-wa-support",
		"version": "1.0.1",
	})
}

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
		"version":     "1.0.1",
		"time":        now.Format("2006-01-02 15:04:05"),
		"timezone":    now.Format("MST"),
		"timestamp":   now.Unix(),
		"message":     "Server is running successfully",
		"environment": gin.Mode(),
	})
}

// GetSessions retrieves WhatsApp sessions with their status
func GetSessions(c *gin.Context) {
	db := database.GetDB()

	var sessions []models.WhatsAppSession

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	userToken := c.Query("user_token")
	sessionState := c.Query("session_state")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Build query with filters
	query := db.Model(&models.WhatsAppSession{})

	if userToken != "" {
		query = query.Where("user_token = ?", userToken)
	}

	if sessionState != "" {
		query = query.Where("session_state = ?", sessionState)
	}

	var count int64
	query.Count(&count)

	if err := query.Limit(limit).Offset(offset).Order("last_activity_at desc").Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    count,
		"page":     page,
		"limit":    limit,
		"pages":    (count + int64(limit) - 1) / int64(limit),
		"filters": map[string]interface{}{
			"user_token":    userToken,
			"session_state": sessionState,
		},
	})
}

// GetSessionQR retrieves the current QR code for a session
func GetSessionQR(c *gin.Context) {
	userToken := c.Param("user_token")
	if userToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_token is required"})
		return
	}

	db := database.GetDB()

	var session models.WhatsAppSession
	if err := db.Where("user_token = ?", userToken).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Check if QR is expired
	if session.QRExpiredAt != nil && time.Now().After(*session.QRExpiredAt) {
		c.JSON(http.StatusGone, gin.H{
			"error":         "QR code has expired",
			"session_state": session.SessionState,
		})
		return
	}

	// Don't return QR if session is already connected
	if session.SessionState == "connected" {
		c.JSON(http.StatusOK, gin.H{
			"session_state": "connected",
			"connected_at":  session.ConnectedAt,
			"message":       "Session is already connected",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_token":    userToken,
		"session_state": session.SessionState,
		"qr_code":       session.QRCode,
		"qr_expires_at": session.QRExpiredAt,
		"last_activity": session.LastActivityAt,
	})
}

// GetMessageStatuses retrieves message delivery/read statuses
func GetMessageStatuses(c *gin.Context) {
	db := database.GetDB()

	var statuses []models.WhatsAppMessageStatus

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	userToken := c.Query("user_token")
	messageID := c.Query("message_id")
	status := c.Query("status")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Build query with filters
	query := db.Model(&models.WhatsAppMessageStatus{})

	if userToken != "" {
		query = query.Where("user_token = ?", userToken)
	}

	if messageID != "" {
		query = query.Where("message_id = ?", messageID)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var count int64
	query.Count(&count)

	if err := query.Limit(limit).Offset(offset).Order("event_timestamp desc").Find(&statuses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve message statuses"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"statuses": statuses,
		"total":    count,
		"page":     page,
		"limit":    limit,
		"pages":    (count + int64(limit) - 1) / int64(limit),
		"filters": map[string]interface{}{
			"user_token": userToken,
			"message_id": messageID,
			"status":     status,
		},
	})
}

// GetChatPresences retrieves chat presence (typing) events
func GetChatPresences(c *gin.Context) {
	db := database.GetDB()

	var presences []models.WhatsAppChatPresence

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	userToken := c.Query("user_token")
	chatJid := c.Query("chat_jid")
	state := c.Query("state")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Build query with filters
	query := db.Model(&models.WhatsAppChatPresence{})

	if userToken != "" {
		query = query.Where("user_token = ?", userToken)
	}

	if chatJid != "" {
		query = query.Where("chat_jid = ?", chatJid)
	}

	if state != "" {
		query = query.Where("state = ?", state)
	}

	var count int64
	query.Count(&count)

	if err := query.Limit(limit).Offset(offset).Order("received_at desc").Find(&presences).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve chat presences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"presences": presences,
		"total":     count,
		"page":      page,
		"limit":     limit,
		"pages":     (count + int64(limit) - 1) / int64(limit),
		"filters": map[string]interface{}{
			"user_token": userToken,
			"chat_jid":   chatJid,
			"state":      state,
		},
	})
}

// SyncSessionStatus syncs session status with WhatsApp server
func SyncSessionStatus(c *gin.Context) {
	serverURL := os.Getenv("WA_SERVER_URL")
	adminToken := os.Getenv("WA_ADMIN_TOKEN")

	if serverURL == "" || adminToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "WhatsApp server configuration not found"})
		return
	}

	// Make request to WA server admin API - using GET method
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/admin/users", serverURL), nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Set("Authorization", adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making request to WA server: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to WhatsApp server"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	var serverResponse models.WhatsAppServerResponse
	if err := json.Unmarshal(body, &serverResponse); err != nil {
		log.Printf("Error parsing response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	if !serverResponse.Success {
		log.Printf("Server returned error: %+v", serverResponse)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server returned error", "response": serverResponse})
		return
	}

	// Update local database with server data
	db := database.GetDB()
	now := time.Now()
	updatedSessions := 0
	createdSessions := 0

	for _, serverUser := range serverResponse.Data {
		var session models.WhatsAppSession
		result := db.Where("user_token = ?", serverUser.Token).First(&session)

		if result.Error != nil {
			// Create new session record
			session = models.WhatsAppSession{
				UserToken:   serverUser.Token,
				SessionName: serverUser.Name,
				SessionID:   serverUser.ID,
				JID:         serverUser.JID,
				Connected:   serverUser.Connected,
				LoggedIn:    serverUser.LoggedIn,
				QRCode:      serverUser.QRCode,
				Webhook:     serverUser.Webhook,
				LastSyncAt:  &now,
			}

			// Set session state based on server status
			if serverUser.Connected && serverUser.LoggedIn {
				session.SessionState = "connected"
				session.ConnectedAt = &now
				session.DisconnectedAt = nil
				session.QRCode = "" // Clear QR when connected
				session.QRExpiredAt = nil
			} else if serverUser.QRCode != "" {
				session.SessionState = "qr_waiting"
				session.QRCode = serverUser.QRCode
				expirationTime := now.Add(60 * time.Second)
				session.QRExpiredAt = &expirationTime
			} else {
				session.SessionState = "disconnected"
				session.DisconnectedAt = &now
			}

			if err := db.Create(&session).Error; err != nil {
				log.Printf("Error creating session for token %s: %v", serverUser.Token, err)
				continue
			}
			createdSessions++
		} else {
			// Update existing session
			session.SessionName = serverUser.Name
			session.SessionID = serverUser.ID
			session.JID = serverUser.JID
			session.Connected = serverUser.Connected
			session.LoggedIn = serverUser.LoggedIn
			session.Webhook = serverUser.Webhook
			session.LastSyncAt = &now

			// Update session state based on server status
			oldState := session.SessionState
			if serverUser.Connected && serverUser.LoggedIn {
				session.SessionState = "connected"
				if oldState != "connected" {
					session.ConnectedAt = &now
				}
				session.DisconnectedAt = nil
				session.QRCode = "" // Clear QR when connected
				session.QRExpiredAt = nil
			} else if serverUser.QRCode != "" {
				session.SessionState = "qr_waiting"
				session.QRCode = serverUser.QRCode
				expirationTime := now.Add(60 * time.Second)
				session.QRExpiredAt = &expirationTime
			} else {
				session.SessionState = "disconnected"
				if oldState != "disconnected" {
					session.DisconnectedAt = &now
				}
			}

			if err := db.Save(&session).Error; err != nil {
				log.Printf("Error updating session for token %s: %v", serverUser.Token, err)
				continue
			}
			updatedSessions++
		}

		// Create or update user settings with default chat log disabled
		var userSettings models.UserSettings
		userResult := db.Where("user_token = ?", serverUser.Token).First(&userSettings)

		if userResult.Error != nil {
			// Create new user settings with chat log disabled by default
			userSettings = models.UserSettings{
				UserToken:      serverUser.Token,
				ChatLogEnabled: false, // Default: disabled when syncing
				IsActive:       true,
				DisplayName:    serverUser.Name,
			}
			if err := db.Create(&userSettings).Error; err != nil {
				log.Printf("Error creating user settings for token %s: %v", serverUser.Token, err)
			} else {
				log.Printf("Created user settings for token %s (chat log disabled)", serverUser.Token)
			}
		} else {
			// Update existing user settings - reset chat log to disabled when syncing
			updates := map[string]interface{}{
				"chat_log_enabled": false, // Reset to disabled on sync
				"is_active":        true,
			}
			if serverUser.Name != "" {
				updates["display_name"] = serverUser.Name
			}

			if err := db.Model(&userSettings).Updates(updates).Error; err != nil {
				log.Printf("Error updating user settings for token %s: %v", serverUser.Token, err)
			} else {
				log.Printf("Updated user settings for token %s (chat log reset to disabled)", serverUser.Token)
			}
		}
	}

	log.Printf("Session sync completed: %d created, %d updated", createdSessions, updatedSessions)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Session status synced successfully",
		"stats": map[string]interface{}{
			"total_sessions":   len(serverResponse.Data),
			"created_sessions": createdSessions,
			"updated_sessions": updatedSessions,
			"last_sync_at":     now,
		},
		"sessions": serverResponse.Data,
	})
}

// processMessageEventWithChatRoom processes message events and creates/updates chat room
func processMessageEventWithChatRoom(webhookData ClivyWebhookData) error {
	// First process the message normally
	if err := processMessageEvent(webhookData); err != nil {
		return err
	}

	// Then create/update chat room and chat message
	return createChatRoomAndMessage(webhookData, false) // false = incoming message
}

// processMessageSentEventWithChatRoom processes sent message events and creates/updates chat room
func processMessageSentEventWithChatRoom(webhookData ClivyWebhookData) error {
	// First process the message normally
	if err := processMessageSentEvent(webhookData); err != nil {
		return err
	}

	// Then create/update chat room and chat message
	return createChatRoomAndMessage(webhookData, true) // true = outgoing message
}

// processReadReceiptEventWithChatRoom processes read receipt events and updates message status
func processReadReceiptEventWithChatRoom(webhookData ClivyWebhookData) error {
	// First process the read receipt normally
	if err := processReadReceiptEvent(webhookData); err != nil {
		return err
	}

	// Then update message status in chat messages
	return updateChatMessageStatus(webhookData)
}

// createChatRoomAndMessage creates/updates chat room and adds chat message
func createChatRoomAndMessage(webhookData ClivyWebhookData, isOutgoing bool) error {
	data := webhookData.Data
	db := database.GetDB()

	// Extract message info from the webhook data
	var messageInfo map[string]interface{}
	var messageData map[string]interface{}

	if info, ok := data["Info"].(map[string]interface{}); ok {
		messageInfo = info
	} else {
		messageInfo = data
	}

	if message, ok := data["Message"].(map[string]interface{}); ok {
		messageData = message
	} else {
		messageData = data
	}

	// Extract basic info
	messageID, _ := messageInfo["ID"].(string)
	if messageID == "" {
		messageID, _ = data["MessageID"].(string)
		if messageID == "" {
			messageID, _ = data["id"].(string)
		}
	}

	var senderJID, recipientJID, contactJID, contactName string
	var senderType string

	if isOutgoing {
		// For outgoing messages, get session owner as sender
		var session models.WhatsAppSession
		if err := db.Where("user_token = ?", webhookData.UserToken).First(&session).Error; err == nil {
			senderJID = session.JID
		}
		recipientJID, _ = messageInfo["Chat"].(string)
		if recipientJID == "" {
			recipientJID, _ = data["to"].(string)
		}
		contactJID = recipientJID
		senderType = "user"
	} else {
		// For incoming messages
		senderJID, _ = messageInfo["Sender"].(string)
		if senderJID == "" {
			senderJID, _ = data["from"].(string)
		}
		// Clean sender format - import the function from user.go
		senderJID = cleanJIDWebhook(senderJID)
		contactJID = senderJID
		senderType = "contact"

		// Get contact name
		contactName, _ = messageInfo["PushName"].(string)
		if contactName == "" {
			contactName, _ = data["pushname"].(string)
		}
	}

	// Extract message content
	messageType, _ := messageInfo["Type"].(string)
	if messageType == "" {
		messageType, _ = data["type"].(string)
	}

	var content, caption string
	var mediaData models.JSONB

	switch messageType {
	case "text":
		if conversation, ok := messageData["conversation"].(string); ok {
			content = conversation
		} else if extendedText, ok := messageData["extendedTextMessage"].(map[string]interface{}); ok {
			if text, ok := extendedText["text"].(string); ok {
				content = text
			}
		} else if bodyText, ok := messageData["body"].(string); ok {
			content = bodyText
		}

	case "image", "video", "audio", "document", "sticker":
		if bodyText, ok := messageData["body"].(string); ok {
			content = bodyText
		}
		if captionText, ok := messageData["caption"].(string); ok {
			caption = captionText
		}
		if media, ok := messageData["media"].(map[string]interface{}); ok {
			mediaData = models.JSONB(media)
		}
	}

	// Parse timestamp
	timestampStr, _ := messageInfo["Timestamp"].(string)
	if timestampStr == "" {
		timestampStr, _ = data["timestamp"].(string)
	}

	messageTimestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		if timestampInt, ok := messageInfo["Timestamp"].(float64); ok {
			messageTimestamp = time.Unix(int64(timestampInt), 0)
		} else {
			messageTimestamp = time.Now()
		}
	}

	// Check if it's a group message
	isGroup, _ := messageInfo["IsGroup"].(bool)

	// Determine last sender for chat room
	var lastSender string
	if isOutgoing {
		lastSender = "user"
	} else {
		lastSender = "contact"
	}

	// Create short preview of message for chat room
	var lastMessage string
	if content != "" {
		if len(content) > 100 {
			lastMessage = content[:100] + "..."
		} else {
			lastMessage = content
		}
	} else {
		lastMessage = fmt.Sprintf("[%s]", messageType)
	}

	// Create or update chat room
	chatRoom, err := createOrUpdateChatRoomWebhook(db, webhookData.UserToken, contactJID, contactName, lastMessage, lastSender, isGroup)
	if err != nil {
		log.Printf("Error creating/updating chat room: %v", err)
		return err
	}

	// Create chat message
	err = createChatMessageWebhook(db, messageID, chatRoom.ChatID, webhookData.UserToken, senderJID, senderType, messageType, content, caption, mediaData, messageTimestamp, "")
	if err != nil {
		log.Printf("Error creating chat message: %v", err)
		return err
	}

	log.Printf("Created chat message %s in room %s", messageID, chatRoom.ChatID)
	return nil
}

// updateChatMessageStatus updates message status in chat messages based on read receipt
func updateChatMessageStatus(webhookData ClivyWebhookData) error {
	data := webhookData.Data
	db := database.GetDB()

	// Extract info from new format structure
	var eventInfo map[string]interface{}

	if event, ok := data["event"].(map[string]interface{}); ok {
		eventInfo = event
	} else {
		eventInfo = data
	}

	// Get status from the webhook data
	status, _ := data["state"].(string)
	if status == "" {
		status, _ := eventInfo["Type"].(string)
		if status == "read" {
			status = "Read"
		}
	}

	// Convert status to lowercase for consistency
	receiptType := strings.ToLower(status)

	// Extract message IDs
	var messageIds []interface{}
	if msgIds, ok := eventInfo["MessageIDs"].([]interface{}); ok {
		messageIds = msgIds
	} else if msgIds, ok := data["messageIds"].([]interface{}); ok {
		messageIds = msgIds
	}

	// Update message status in chat messages
	for _, msgId := range messageIds {
		if messageID, ok := msgId.(string); ok {
			err := updateMessageStatusWebhook(db, messageID, receiptType)
			if err != nil {
				log.Printf("Error updating chat message status: %v", err)
			}
		}
	}

	return nil
}

// Helper functions for webhook processing
func cleanJIDWebhook(jid string) string {
	// Remove device suffix (:24, :26) from JID
	if strings.Contains(jid, ":") {
		parts := strings.Split(jid, ":")
		if len(parts) > 0 {
			jid = parts[0]
		}
	}

	// Ensure proper format
	if !strings.Contains(jid, "@") {
		jid = jid + "@s.whatsapp.net"
	}

	return jid
}

func generateChatIDWebhook(userToken, contactJID string) string {
	return fmt.Sprintf("%s_%s", userToken, contactJID)
}

func createOrUpdateChatRoomWebhook(db *gorm.DB, userToken, contactJID, contactName, lastMessage, lastSender string, isGroup bool) (*models.ChatRoom, error) {
	chatID := generateChatIDWebhook(userToken, contactJID)

	var chatRoom models.ChatRoom
	result := db.Where("chat_id = ?", chatID).First(&chatRoom)

	if result.Error != nil {
		// Create new chat room
		chatRoom = models.ChatRoom{
			ChatID:       chatID,
			UserToken:    userToken,
			ContactJID:   contactJID,
			ContactName:  contactName,
			IsGroup:      isGroup,
			LastMessage:  lastMessage,
			LastSender:   lastSender,
			LastActivity: time.Now(),
			UnreadCount:  0,
		}

		if isGroup {
			chatRoom.ChatType = "group"
			chatRoom.GroupName = contactName
		} else {
			chatRoom.ChatType = "individual"
		}

		return &chatRoom, db.Create(&chatRoom).Error
	} else {
		// Update existing chat room
		updates := map[string]interface{}{
			"last_message":  lastMessage,
			"last_sender":   lastSender,
			"last_activity": time.Now(),
		}

		if contactName != "" {
			if isGroup {
				updates["group_name"] = contactName
			} else {
				updates["contact_name"] = contactName
			}
		}

		// Increment unread count if message is from contact
		if lastSender == "contact" {
			updates["unread_count"] = chatRoom.UnreadCount + 1
		}

		err := db.Model(&chatRoom).Updates(updates).Error
		if err != nil {
			return nil, err
		}

		// Refresh the model
		db.Where("chat_id = ?", chatID).First(&chatRoom)
		return &chatRoom, nil
	}
}

func createChatMessageWebhook(db *gorm.DB, messageID, chatID, userToken, senderJID, senderType, messageType, content, caption string, mediaData models.JSONB, messageTimestamp time.Time, quotedMessageID string) error {
	message := models.ChatMessage{
		MessageID:        messageID,
		ChatID:           chatID,
		UserToken:        userToken,
		SenderJID:        senderJID,
		SenderType:       senderType,
		MessageType:      messageType,
		Content:          content,
		Caption:          caption,
		MediaData:        mediaData,
		QuotedMessageID:  quotedMessageID,
		Status:           "sent",
		MessageTimestamp: messageTimestamp,
	}

	// Set initial status based on sender type
	if senderType == "user" {
		message.Status = "sent"
	} else {
		message.Status = "delivered" // Incoming messages are automatically delivered
	}

	return db.Create(&message).Error
}

func updateMessageStatusWebhook(db *gorm.DB, messageID, status string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status": status,
	}

	switch status {
	case "delivered":
		updates["delivered_at"] = &now
	case "read":
		updates["read_at"] = &now
	}

	return db.Model(&models.ChatMessage{}).
		Where("message_id = ?", messageID).
		Updates(updates).Error
}
