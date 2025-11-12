package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetUserChats retrieves chat rooms for a specific user
func GetUserChats(c *gin.Context) {
	userToken := c.Param("user_token")
	if userToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_token is required"})
		return
	}

	db := database.GetDB()

	// Check if user exists and chat log is enabled
	var userSettings models.UserSettings
	if err := db.Where("user_token = ?", userToken).First(&userSettings).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if !userSettings.ChatLogEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Chat log is disabled for this user"})
		return
	}

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")
	search := c.Query("search")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Build query with search
	query := db.Model(&models.ChatRoom{}).Where("user_token = ?", userToken)

	if search != "" {
		query = query.Where(
			"contact_name ILIKE ? OR contact_jid ILIKE ? OR group_name ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%",
		)
	}

	var count int64
	query.Count(&count)

	var chatRooms []models.ChatRoom
	if err := query.
		Limit(limit).
		Offset(offset).
		Order("last_activity desc").
		Find(&chatRooms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve chat rooms"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"chat_rooms": chatRooms,
		"total":      count,
		"page":       page,
		"limit":      limit,
		"pages":      (count + int64(limit) - 1) / int64(limit),
		"filters": map[string]interface{}{
			"search": search,
		},
	})
}

// GetChatMessages retrieves messages for a specific chat
func GetChatMessages(c *gin.Context) {
	userToken := c.Param("user_token")
	chatID := c.Param("chat_id")

	if userToken == "" || chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_token and chat_id are required"})
		return
	}

	db := database.GetDB()

	// Check if user exists and chat log is enabled
	var userSettings models.UserSettings
	if err := db.Where("user_token = ?", userToken).First(&userSettings).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if !userSettings.ChatLogEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Chat log is disabled for this user"})
		return
	}

	// Verify chat room belongs to user
	var chatRoom models.ChatRoom
	if err := db.Where("chat_id = ? AND user_token = ?", chatID, userToken).First(&chatRoom).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chat room not found"})
		return
	}

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")
	messageType := c.Query("message_type")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	offset := (page - 1) * limit

	// Build query
	query := db.Model(&models.ChatMessage{}).Where("chat_id = ? AND user_token = ?", chatID, userToken)

	if messageType != "" {
		query = query.Where("message_type = ?", messageType)
	}

	var count int64
	query.Count(&count)

	var messages []models.ChatMessage
	if err := query.
		Limit(limit).
		Offset(offset).
		Order("message_timestamp desc").
		Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve messages"})
		return
	}

	// Mark unread messages as read if auto_read is enabled
	if userSettings.AutoReadEnabled {
		go markMessagesAsRead(db, chatID, userToken)
	}

	c.JSON(http.StatusOK, gin.H{
		"messages":  messages,
		"chat_room": chatRoom,
		"total":     count,
		"page":      page,
		"limit":     limit,
		"pages":     (count + int64(limit) - 1) / int64(limit),
		"filters": map[string]interface{}{
			"message_type": messageType,
		},
	})
}

// GetUserEvents retrieves events for a specific user
func GetUserEvents(c *gin.Context) {
	userToken := c.Param("user_token")
	if userToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_token is required"})
		return
	}

	db := database.GetDB()

	// Check if user exists
	var userSettings models.UserSettings
	if err := db.Where("user_token = ?", userToken).First(&userSettings).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")
	eventType := c.Query("event_type")
	source := c.DefaultQuery("source", "wa")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	query := db.Model(&models.GenEventWebhook{}).
		Where("user_token = ? AND source = ?", userToken, source)

	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	var count int64
	query.Count(&count)

	var events []models.GenEventWebhook
	if err := query.
		Limit(limit).
		Offset(offset).
		Order("received_at desc").
		Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve events"})
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
		},
	})
}

// markMessagesAsRead marks all unread messages in a chat as read
func markMessagesAsRead(db *gorm.DB, chatID, userToken string) {
	now := time.Now()

	// Update chat messages status to read
	db.Model(&models.ChatMessage{}).
		Where("chat_id = ? AND user_token = ? AND sender_type = ? AND status != ?",
			chatID, userToken, "contact", "read").
		Updates(map[string]interface{}{
			"status":  "read",
			"read_at": &now,
		})

	// Reset unread count for chat room
	db.Model(&models.ChatRoom{}).
		Where("chat_id = ? AND user_token = ?", chatID, userToken).
		Update("unread_count", 0)
}

// Helper function to generate chat ID
func generateChatID(userToken, contactJID string) string {
	return fmt.Sprintf("%s_%s", userToken, contactJID)
}

// Helper function to clean JID format
func cleanJID(jid string) string {
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

// Helper function to determine sender type
func getSenderType(senderJID, userJID string) string {
	if senderJID == userJID {
		return "user"
	}
	return "contact"
}

// CreateOrUpdateChatRoom creates or updates a chat room
func CreateOrUpdateChatRoom(db *gorm.DB, userToken, contactJID, contactName, lastMessage, lastSender string, isGroup bool) (*models.ChatRoom, error) {
	chatID := generateChatID(userToken, contactJID)

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

// CreateChatMessage creates a new chat message
func CreateChatMessage(db *gorm.DB, messageID, chatID, userToken, senderJID, senderType, messageType, content, caption string, mediaData models.JSONB, messageTimestamp time.Time, quotedMessageID string) error {
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

// UpdateMessageStatus updates the status of a message
func UpdateMessageStatus(db *gorm.DB, messageID, status string) error {
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
