package handlers

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"genfity-wa-support/database"
	"genfity-wa-support/models"

	"github.com/gin-gonic/gin"
)

// AdminAuthMiddleware checks for admin bearer token
func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		adminToken := os.Getenv("ADMIN_TOKEN")
		if adminToken == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Admin token not configured"})
			c.Abort()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != adminToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid admin token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminGetUsers retrieves all users and their settings
func AdminGetUsers(c *gin.Context) {
	db := database.GetDB()

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

	// Join user settings with session data
	type UserWithStats struct {
		UserToken       string `json:"user_token"`
		ChatLogEnabled  bool   `json:"chat_log_enabled"`
		AutoReadEnabled bool   `json:"auto_read_enabled"`
		DisplayName     string `json:"display_name"`
		IsActive        bool   `json:"is_active"`
		SessionState    string `json:"session_state"`
		Connected       bool   `json:"connected"`
		LoggedIn        bool   `json:"logged_in"`
		JID             string `json:"jid"`
		MessageCount    int64  `json:"message_count"`
		ChatRoomCount   int64  `json:"chat_room_count"`
		LastActivity    string `json:"last_activity"`
	}

	var users []UserWithStats

	// Query to get all users with their stats using proper GORM table names
	err := db.Table("user_settings us").
		Select(`
			us.user_token,
			us.chat_log_enabled,
			us.auto_read_enabled,
			us.display_name,
			COALESCE(ws.j_id, '') as phone_number,
			us.is_active,
			COALESCE(ws.session_state, 'unknown') as session_state,
			COALESCE(ws.connected, false) as connected,
			COALESCE(ws.logged_in, false) as logged_in,
			COALESCE(ws.j_id, '') as jid,
			COALESCE(msg_count.count, 0) as message_count,
			COALESCE(chat_count.count, 0) as chat_room_count,
			COALESCE(ws.last_activity_at::text, '') as last_activity
		`).
		Joins("LEFT JOIN whats_app_sessions ws ON us.user_token = ws.user_token").
		Joins(`LEFT JOIN (
			SELECT user_token, COUNT(*) as count 
			FROM chat_messages 
			GROUP BY user_token
		) msg_count ON us.user_token = msg_count.user_token`).
		Joins(`LEFT JOIN (
			SELECT user_token, COUNT(*) as count 
			FROM chat_rooms 
			GROUP BY user_token
		) chat_count ON us.user_token = chat_count.user_token`).
		Order("us.updated_at desc").
		Limit(limit).
		Offset(offset).
		Find(&users).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	// Get total count
	var totalCount int64
	db.Model(&models.UserSettings{}).Count(&totalCount)

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": totalCount,
		"page":  page,
		"limit": limit,
		"pages": (totalCount + int64(limit) - 1) / int64(limit),
	})
}

// AdminGetUser retrieves specific user details
func AdminGetUser(c *gin.Context) {
	userToken := c.Param("user_token")
	if userToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_token is required"})
		return
	}

	db := database.GetDB()

	// Get user settings
	var userSettings models.UserSettings
	if err := db.Where("user_token = ?", userToken).First(&userSettings).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get session data
	var session models.WhatsAppSession
	db.Where("user_token = ?", userToken).First(&session)

	// Get statistics
	var messageCount, chatCount, unreadCount int64
	db.Model(&models.ChatMessage{}).Where("user_token = ?", userToken).Count(&messageCount)
	db.Model(&models.ChatRoom{}).Where("user_token = ?", userToken).Count(&chatCount)
	db.Model(&models.ChatRoom{}).Where("user_token = ? AND unread_count > 0", userToken).Count(&unreadCount)

	c.JSON(http.StatusOK, gin.H{
		"user_settings": userSettings,
		"session":       session,
		"stats": map[string]interface{}{
			"message_count":     messageCount,
			"chat_room_count":   chatCount,
			"unread_chat_count": unreadCount,
		},
	})
}

// AdminUpdateUser updates user settings
func AdminUpdateUser(c *gin.Context) {
	userToken := c.Param("user_token")
	if userToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_token is required"})
		return
	}

	db := database.GetDB()

	var updateData struct {
		ChatLogEnabled  *bool   `json:"chat_log_enabled"`
		AutoReadEnabled *bool   `json:"auto_read_enabled"`
		WebhookURL      *string `json:"webhook_url"`
		DisplayName     *string `json:"display_name"`
		IsActive        *bool   `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data"})
		return
	}

	// Find or create user settings
	var userSettings models.UserSettings
	result := db.Where("user_token = ?", userToken).First(&userSettings)

	if result.Error != nil {
		// Create new user settings
		userSettings = models.UserSettings{
			UserToken: userToken,
		}
	}

	// Update fields if provided
	if updateData.ChatLogEnabled != nil {
		userSettings.ChatLogEnabled = *updateData.ChatLogEnabled
	}
	if updateData.AutoReadEnabled != nil {
		userSettings.AutoReadEnabled = *updateData.AutoReadEnabled
	}
	if updateData.WebhookURL != nil {
		userSettings.WebhookURL = *updateData.WebhookURL
	}
	if updateData.DisplayName != nil {
		userSettings.DisplayName = *updateData.DisplayName
	}
	if updateData.IsActive != nil {
		userSettings.IsActive = *updateData.IsActive
	}

	// Save or create
	if result.Error != nil {
		if err := db.Create(&userSettings).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user settings"})
			return
		}
	} else {
		if err := db.Save(&userSettings).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user settings"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"message":       "User settings updated successfully",
		"user_settings": userSettings,
	})
}

// AdminGetSessions retrieves all WhatsApp sessions
func AdminGetSessions(c *gin.Context) {
	db := database.GetDB()

	var sessions []models.WhatsAppSession

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
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
			"session_state": sessionState,
		},
	})
}

// AdminGetUserSessions retrieves sessions for specific user
func AdminGetUserSessions(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{
		"session": session,
	})
}

// AdminGetEvents retrieves all events across all users
func AdminGetEvents(c *gin.Context) {
	db := database.GetDB()

	var events []models.GenEventWebhook

	// Pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	eventType := c.Query("event_type")
	source := c.DefaultQuery("source", "wa")
	userToken := c.Query("user_token")

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
			"user_token": userToken,
		},
	})
}
