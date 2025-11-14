package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/models"
	"genfity-wa-support/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SendImageRequest represents the request body for sending images
type SendImageRequest struct {
	Phone   string `json:"Phone"`
	Image   string `json:"Image"`
	Caption string `json:"Caption"`
}

// Global endpoints that don't require token validation
var globalEndpoints = []string{
	"/webhook/events", // WhatsApp calls this endpoint
	"/health",
}

// isGlobalEndpoint checks if the given path is a global endpoint
func isGlobalEndpoint(path string) bool {
	for _, endpoint := range globalEndpoints {
		if path == endpoint || strings.HasPrefix(path, endpoint+"/") {
			return true
		}
	}
	return false
}

// isDataURI checks if a string is a data URI (base64 encoded)
func isDataURI(s string) bool {
	return strings.HasPrefix(s, "data:")
}

// isValidURL checks if a string is a valid URL
func isValidURL(s string) bool {
	_, err := url.ParseRequestURI(s)
	return err == nil && (strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://"))
}

// getMimeTypeFromBytes detects MIME type from image bytes
func getMimeTypeFromBytes(data []byte) string {
	if len(data) < 8 {
		return "application/octet-stream"
	}

	// Check PNG signature
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png"
	}

	// Check JPEG signature
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return "image/jpeg"
	}

	// Check GIF signature
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return "image/gif"
	}

	// Check WebP signature
	if len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}

	// Default fallback
	return "image/jpeg"
}

// downloadAndEncodeImage downloads image from URL and converts to base64 data URI
func downloadAndEncodeImage(imageURL string) (string, error) {
	log.Printf("DEBUG: Downloading image from URL: %s", imageURL)

	// Create HTTP client with timeout
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download image: HTTP %d", resp.StatusCode)
	}

	// Read image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read image data: %v", err)
	}

	// Detect MIME type
	mimeType := getMimeTypeFromBytes(imageData)
	log.Printf("DEBUG: Detected MIME type: %s", mimeType)

	// Encode to base64
	base64Data := base64.StdEncoding.EncodeToString(imageData)

	// Create data URI
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

	log.Printf("DEBUG: Successfully converted image to data URI (length: %d)", len(dataURI))

	return dataURI, nil
}

// processImageRequest processes request body for /chat/send/image endpoint
func processImageRequest(bodyBytes []byte) ([]byte, error) {
	var request SendImageRequest
	if err := json.Unmarshal(bodyBytes, &request); err != nil {
		return bodyBytes, nil // If can't parse, return original
	}

	// Check if Image field is a URL that needs to be converted
	if request.Image != "" && !isDataURI(request.Image) && isValidURL(request.Image) {
		log.Printf("DEBUG: Converting URL to base64: %s", request.Image)

		dataURI, err := downloadAndEncodeImage(request.Image)
		if err != nil {
			log.Printf("ERROR: Failed to convert image URL: %v", err)
			return nil, err
		}

		// Update the Image field with the data URI
		request.Image = dataURI

		// Re-encode to JSON
		modifiedBytes, err := json.Marshal(request)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal modified request: %v", err)
		}

		log.Printf("DEBUG: Successfully modified request body")
		return modifiedBytes, nil
	}

	// Return original if no conversion needed
	return bodyBytes, nil
}

// WhatsAppGateway handles all WhatsApp API requests with /wa prefix
func WhatsAppGateway(c *gin.Context) {
	path := c.Request.URL.Path
	method := c.Request.Method

	// log.Printf("DEBUG: Gateway received request - Method: %s, Path: %s", method, path)

	// Remove /wa prefix to get actual WA server path
	actualPath := strings.TrimPrefix(path, "/wa")
	// log.Printf("DEBUG: Actual path after prefix removal: %s", actualPath)

	// Admin routes bypass all validation
	if strings.HasPrefix(actualPath, "/admin") {
		// log.Printf("DEBUG: Admin route detected, bypassing validation")
		proxyToWAServer(c, actualPath)
		return
	}

	// Global endpoints that don't require token validation
	if isGlobalEndpoint(actualPath) {
		log.Printf("DEBUG: Global endpoint detected, bypassing token validation")
		proxyToWAServer(c, actualPath)
		return
	}

	// For non-admin routes, validate token and subscription
	token := getTokenFromRequest(c)
	if token == "" {
		log.Printf("DEBUG: No token provided")
		c.JSON(http.StatusUnauthorized, models.GatewayResponse{
			Status:  http.StatusUnauthorized,
			Message: "Token required",
		})
		return
	}

	log.Printf("DEBUG: Token received: %s", token)

	// Validate token and subscription
	userID, err := validateTokenAndSubscription(token, actualPath)
	if err != nil {
		log.Printf("Validation failed: %v", err)
		c.JSON(http.StatusForbidden, models.GatewayResponse{
			Status:  http.StatusForbidden,
			Message: err.Error(),
		})
		return
	}

	// If this is a session connect request, check session limits
	if actualPath == "/session/connect" && method == "POST" {
		if err := checkSessionLimits(userID); err != nil {
			c.JSON(http.StatusForbidden, models.GatewayResponse{
				Status:  http.StatusForbidden,
				Message: err.Error(),
			})
			return
		}
	}

	// Proxy to WhatsApp server with special handling for image endpoints
	statusCode := proxyToWAServerWithProcessing(c, actualPath)

	// Track message stats based on success/failure
	if isMessageEndpoint(actualPath) && method == "POST" {
		go trackMessageStats(userID, token, actualPath, c, statusCode >= 200 && statusCode < 300)
	}
}

// getTokenFromRequest extracts token from Authorization header or token header
func getTokenFromRequest(c *gin.Context) string {
	// Check token header first (as per API documentation)
	token := c.GetHeader("token")
	if token != "" {
		return token
	}

	// Check Authorization header as fallback
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// Remove "Bearer " prefix if present
		if strings.HasPrefix(authHeader, "Bearer ") {
			return strings.TrimPrefix(authHeader, "Bearer ")
		}
		return authHeader
	}

	return ""
}

// validateTokenAndSubscription validates token and checks subscription status
func validateTokenAndSubscription(token, path string) (string, error) {
	// Find session by token in WhatsAppSession table
	var session models.WhatsappSession
	if err := database.TransactionalDB.Where("token = ?", token).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("invalid token")
		}
		return "", fmt.Errorf("database error: %v", err)
	}

	// Check if session has associated user
	if session.UserID == nil {
		return "", fmt.Errorf("session not associated with any user")
	}

	// Get user's active subscription from ServicesWhatsappCustomers
	var subscription models.ServicesWhatsappCustomers
	err := database.TransactionalDB.
		Where("\"customerId\" = ? AND status = ?", *session.UserID, "active").
		First(&subscription).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("no active subscription found")
		}
		return "", fmt.Errorf("subscription check failed: %v", err)
	}

	// Check if subscription is expired and auto-update status
	if time.Now().After(subscription.ExpiredAt) {
		subscription.Status = "expired"
		database.TransactionalDB.Save(&subscription)
		return "", fmt.Errorf("subscription expired on %s", subscription.ExpiredAt.Format("2006-01-02"))
	}

	return *session.UserID, nil
}

// checkSessionLimits validates session limits for connect requests
func checkSessionLimits(userID string) error {
	// Get user's subscription
	var subscription models.ServicesWhatsappCustomers
	err := database.TransactionalDB.
		Where("\"customerId\" = ? AND status = ?", userID, "active").
		First(&subscription).Error
	if err != nil {
		return fmt.Errorf("no active subscription found")
	}

	// Get package info
	var packageInfo models.WhatsappApiPackage
	err = database.TransactionalDB.Where("id = ?", subscription.PackageID).First(&packageInfo).Error
	if err != nil {
		return fmt.Errorf("package not found")
	}

	// Count current active sessions for this user
	var currentSessions int64
	database.TransactionalDB.Model(&models.WhatsappSession{}).
		Where("\"userId\" = ? AND connected = ?", userID, true).
		Count(&currentSessions)

	// Check if adding new session would exceed limit
	if int(currentSessions) >= packageInfo.MaxSession {
		return fmt.Errorf("session limit exceeded. Maximum allowed: %d, current: %d",
			packageInfo.MaxSession, currentSessions)
	}

	return nil
}

// proxyToWAServerWithProcessing forwards the request to WhatsApp server with optional body processing
func proxyToWAServerWithProcessing(c *gin.Context, targetPath string) int {
	// Handle typing indicator and auto-read before sending message
	if isMessageEndpoint(targetPath) && c.Request.Method == "POST" {
		token := getTokenFromRequest(c)
		if token != "" {
			// Handle typing indicator (regular chat only, not AI)
			handleTypingIndicatorBeforeSend(token, c)

			// Auto-read incoming messages before sending (if enabled)
			handleAutoReadBeforeSend(token, c)
		}
	}

	// Check if this is an image endpoint that needs special processing
	if targetPath == "/chat/send/image" && c.Request.Method == "POST" {
		return proxyImageRequest(c, targetPath)
	}

	// For all other endpoints, use normal proxy
	statusCode := proxyToWAServer(c, targetPath)

	// Stop typing indicator and save outgoing message after successful send
	if isMessageEndpoint(targetPath) && c.Request.Method == "POST" && statusCode >= 200 && statusCode < 300 {
		token := getTokenFromRequest(c)
		if token != "" {
			// Stop typing indicator (regular chat only)
			go handleTypingIndicatorAfterSend(token, c)

			// Save outgoing message to DB
			go handleSaveOutgoingMessage(token, c, statusCode)
		}
	}

	return statusCode
}

// proxyImageRequest handles image endpoint with URL to base64 conversion
func proxyImageRequest(c *gin.Context, targetPath string) int {
	waServerURL := os.Getenv("WA_SERVER_URL")
	if waServerURL == "" {
		c.JSON(http.StatusInternalServerError, models.GatewayResponse{
			Status:  http.StatusInternalServerError,
			Message: "WhatsApp server URL not configured",
		})
		return http.StatusInternalServerError
	}

	// Read request body
	var bodyBytes []byte
	if c.Request.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.GatewayResponse{
				Status:  http.StatusInternalServerError,
				Message: "Failed to read request body",
			})
			return http.StatusInternalServerError
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Process image request (convert URL to base64 if needed)
	processedBody, err := processImageRequest(bodyBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.GatewayResponse{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("Failed to process image: %v", err),
		})
		return http.StatusBadRequest
	}

	// Create new request to WA server
	targetURL := waServerURL + targetPath
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	log.Printf("DEBUG: Proxying image request to URL: %s", targetURL)
	log.Printf("DEBUG: Request method: %s", c.Request.Method)
	log.Printf("DEBUG: Original body length: %d, Processed body length: %d", len(bodyBytes), len(processedBody))

	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(processedBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GatewayResponse{
			Status:  http.StatusInternalServerError,
			Message: "Failed to create request",
		})
		return http.StatusInternalServerError
	}

	// Copy all headers exactly as received
	for name, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Update Content-Length if body was modified
	if len(processedBody) != len(bodyBytes) {
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(processedBody)))
	}

	// Execute request to WA server
	client := &http.Client{Timeout: 60 * time.Second} // Longer timeout for image processing
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, models.GatewayResponse{
			Status:  http.StatusBadGateway,
			Message: "Failed to reach WhatsApp server",
		})
		return http.StatusBadGateway
	}
	defer resp.Body.Close()

	// Read response from WA server
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GatewayResponse{
			Status:  http.StatusInternalServerError,
			Message: "Failed to read response",
		})
		return http.StatusInternalServerError
	}

	// Copy response headers exactly
	for name, values := range resp.Header {
		for _, value := range values {
			c.Header(name, value)
		}
	}

	// Return response with same status code and body as WA server
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), responseBody)

	return resp.StatusCode
}

// proxyToWAServer forwards the request to WhatsApp server without modification
func proxyToWAServer(c *gin.Context, targetPath string) int {
	waServerURL := os.Getenv("WA_SERVER_URL")
	if waServerURL == "" {
		c.JSON(http.StatusInternalServerError, models.GatewayResponse{
			Status:  http.StatusInternalServerError,
			Message: "WhatsApp server URL not configured",
		})
		return http.StatusInternalServerError
	}

	// Read request body
	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Transform request body for message endpoints (convert our format to WA server format)
	if isMessageEndpoint(targetPath) {
		transformedBody, err := transformMessageRequest(bodyBytes, targetPath)
		if err != nil {
			log.Printf("⚠️  Failed to transform request: %v", err)
			c.JSON(http.StatusBadRequest, models.GatewayResponse{
				Status:  http.StatusBadRequest,
				Message: "Invalid request format",
			})
			return http.StatusBadRequest
		}
		bodyBytes = transformedBody
	}

	// Create new request to WA server using the stripped path
	targetURL := waServerURL + targetPath
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	log.Printf("DEBUG: Proxying to URL: %s", targetURL)
	log.Printf("DEBUG: Request method: %s", c.Request.Method)
	log.Printf("DEBUG: Transformed body: %s", string(bodyBytes))

	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GatewayResponse{
			Status:  http.StatusInternalServerError,
			Message: "Failed to create request",
		})
		return http.StatusInternalServerError
	}

	// Copy all headers exactly as received
	for name, values := range c.Request.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// Execute request to WA server
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, models.GatewayResponse{
			Status:  http.StatusBadGateway,
			Message: "Failed to reach WhatsApp server",
		})
		return http.StatusBadGateway
	}
	defer resp.Body.Close()

	// Read response from WA server
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GatewayResponse{
			Status:  http.StatusInternalServerError,
			Message: "Failed to read response",
		})
		return http.StatusInternalServerError
	}

	// Copy response headers exactly
	for name, values := range resp.Header {
		for _, value := range values {
			c.Header(name, value)
		}
	}

	// Return response with same status code and body as WA server
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), responseBody)

	return resp.StatusCode
}

// isMessageEndpoint checks if the endpoint is a message sending endpoint
func isMessageEndpoint(path string) bool {
	messageEndpoints := []string{
		"/chat/send/text",
		"/chat/send/image",
		"/chat/send/audio",
		"/chat/send/document",
		"/chat/send/video",
		"/chat/send/sticker",
		"/chat/send/location",
		"/chat/send/contact",
		"/chat/send/template",
		"/chat/send/edit",
		"/chat/send/poll",
	}

	for _, endpoint := range messageEndpoints {
		if path == endpoint {
			return true
		}
	}
	return false
}

// extractMessageTypeFromPath extracts message type from the API path
func extractMessageTypeFromPath(path string) string {
	// Remove /wa prefix if present
	path = strings.TrimPrefix(path, "/wa")

	// Extract message type from paths like /chat/send/text, /chat/send/image, etc.
	if strings.Contains(path, "/chat/send/") {
		parts := strings.Split(path, "/")
		if len(parts) >= 4 {
			messageType := parts[3] // text, image, document, audio, etc.
			return messageType
		}
	}

	// Default to text if can't determine
	return "text"
}

// transformMessageRequest converts our API format to WA server format
// Our format: {"sessionId": "xxx", "to": "6281...", "text": "hello"}
// WA server format: {"Phone": "6281...", "Body": "hello"}
func transformMessageRequest(bodyBytes []byte, targetPath string) ([]byte, error) {
	// Parse our format
	var ourFormat map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &ourFormat); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	// Convert to WA server format based on endpoint
	waFormat := make(map[string]interface{})

	// Common field: Phone (from "to" field, strip @s.whatsapp.net suffix)
	if to, ok := ourFormat["to"].(string); ok {
		phone := strings.TrimSuffix(to, "@s.whatsapp.net")
		waFormat["Phone"] = phone
	} else {
		return nil, fmt.Errorf("missing 'to' field")
	}

	// Message type specific fields
	messageType := extractMessageTypeFromPath(targetPath)
	switch messageType {
	case "text":
		if text, ok := ourFormat["text"].(string); ok {
			waFormat["Body"] = text
		} else {
			return nil, fmt.Errorf("missing 'text' field")
		}
	case "image", "video", "document", "audio", "sticker":
		// For media: {"Phone": "...", "Body": "caption", "FileName": "..."}
		if caption, ok := ourFormat["caption"].(string); ok {
			waFormat["Body"] = caption
		}
		if fileName, ok := ourFormat["fileName"].(string); ok {
			waFormat["FileName"] = fileName
		}
		if fileURL, ok := ourFormat["fileUrl"].(string); ok {
			waFormat["FileURL"] = fileURL
		}
	case "location":
		// {"Phone": "...", "Latitude": ..., "Longitude": ...}
		if lat, ok := ourFormat["latitude"]; ok {
			waFormat["Latitude"] = lat
		}
		if lon, ok := ourFormat["longitude"]; ok {
			waFormat["Longitude"] = lon
		}
		if name, ok := ourFormat["name"].(string); ok {
			waFormat["Name"] = name
		}
	case "contact":
		// {"Phone": "...", "ContactName": "...", "ContactPhone": "..."}
		if name, ok := ourFormat["contactName"].(string); ok {
			waFormat["ContactName"] = name
		}
		if phone, ok := ourFormat["contactPhone"].(string); ok {
			waFormat["ContactPhone"] = phone
		}
	}

	// Marshal back to JSON
	return json.Marshal(waFormat)
}

func trackMessageStats(userID, token, path string, c *gin.Context, success bool) {
	// Extract message type from path
	messageType := extractMessageTypeFromPath(path)

	// Find session by token to get sessionId
	var session models.WhatsappSession
	if err := database.TransactionalDB.Where("token = ?", token).First(&session).Error; err != nil {
		log.Printf("Failed to find session for token: %v", err)
		return
	}

	// Try to get existing stats record or create new one
	var stats models.WhatsAppMessageStats
	err := database.TransactionalDB.Where("\"userId\" = ? AND \"sessionId\" = ?", userID, session.SessionID).First(&stats).Error

	now := time.Now()

	if err == gorm.ErrRecordNotFound {
		// Create new stats record
		stats = models.WhatsAppMessageStats{
			ID:        uuid.New().String(),
			UserID:    userID,
			SessionID: session.SessionID,
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Initialize counters based on success/failure
		if success {
			stats.TotalMessagesSent = 1
			updateMessageTypeCounter(&stats, messageType, true)
			stats.LastMessageSentAt = &now
		} else {
			stats.TotalMessagesFailed = 1
			updateMessageTypeCounter(&stats, messageType, false)
			stats.LastMessageFailedAt = &now
		}

		if err := database.TransactionalDB.Create(&stats).Error; err != nil {
			log.Printf("Failed to create message stats: %v", err)
		}
	} else if err == nil {
		// Update existing stats record
		if success {
			stats.TotalMessagesSent++
			updateMessageTypeCounter(&stats, messageType, true)
			stats.LastMessageSentAt = &now
		} else {
			stats.TotalMessagesFailed++
			updateMessageTypeCounter(&stats, messageType, false)
			stats.LastMessageFailedAt = &now
		}
		stats.UpdatedAt = now

		if err := database.TransactionalDB.Save(&stats).Error; err != nil {
			log.Printf("Failed to update message stats: %v", err)
		}
	} else {
		log.Printf("Failed to query message stats: %v", err)
	}
}

// handleTypingIndicatorBeforeSend shows typing indicator if enabled (regular chat only, not AI)
func handleTypingIndicatorBeforeSend(sessionToken string, c *gin.Context) {
	// 1. Get session from transactional DB to check typingIndicator setting
	var session models.WhatsappSession
	if err := database.TransactionalDB.Where("token = ?", sessionToken).First(&session).Error; err != nil {
		return // Silently skip if session not found
	}

	// 2. Check if typingIndicator is enabled
	if !session.TypingIndicator {
		return // Typing indicator disabled, skip
	}

	// 3. Parse request body to get target phone number
	bodyBytes, err := c.GetRawData()
	if err != nil {
		return // Silently skip on error
	}

	// Restore body for next handler
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var reqData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
		return // Silently skip on error
	}

	// Extract phone number from "to" field
	toField, ok := reqData["to"].(string)
	if !ok {
		return // No recipient, skip
	}

	// Clean phone number (remove @s.whatsapp.net suffix)
	phoneNumber := strings.TrimSuffix(toField, "@s.whatsapp.net")

	// 4. Set typing state to "composing"
	if err := services.SetTypingState(sessionToken, phoneNumber, "composing"); err != nil {
		log.Printf("⚠️  Failed to set typing state to composing: %v", err)
		// Continue even if typing indicator fails
	}

	// 5. Delay 500ms before sending message
	time.Sleep(500 * time.Millisecond)

	// Note: "stop" typing state will be called after message is sent in proxyToWAServerWithProcessing
}

// handleTypingIndicatorAfterSend stops typing indicator after message is sent
func handleTypingIndicatorAfterSend(sessionToken string, c *gin.Context) {
	// 1. Get session to check if typing indicator was enabled
	var session models.WhatsappSession
	if err := database.TransactionalDB.Where("token = ?", sessionToken).First(&session).Error; err != nil {
		return
	}

	if !session.TypingIndicator {
		return // Not enabled, skip
	}

	// 2. Parse request body to get recipient phone
	bodyBytes, err := c.GetRawData()
	if err != nil {
		return
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
		return
	}

	toField, ok := reqData["to"].(string)
	if !ok {
		return
	}

	phoneNumber := strings.TrimSuffix(toField, "@s.whatsapp.net")

	// 3. Set typing state to "stop"
	if err := services.SetTypingState(sessionToken, phoneNumber, "stop"); err != nil {
		log.Printf("⚠️  Failed to set typing state to stop: %v", err)
	}
}

// handleAutoReadBeforeSend checks if auto-read is enabled and marks unread messages as read
func handleAutoReadBeforeSend(sessionToken string, c *gin.Context) {
	// 1. Get session from transactional DB to check autoReadMessages setting
	var session models.WhatsappSession
	if err := database.TransactionalDB.Where("token = ?", sessionToken).First(&session).Error; err != nil {
		log.Printf("⚠️  Failed to get session for auto-read: %v", err)
		return
	}

	// 2. Check if autoReadMessages is enabled
	if !session.AutoReadMessages {
		return // Auto-read disabled, skip
	}

	// 3. Parse request body to get target phone number
	bodyBytes, err := c.GetRawData()
	if err != nil {
		log.Printf("⚠️  Failed to read request body for auto-read: %v", err)
		return
	}

	// Restore body for next handler
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var reqData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
		log.Printf("⚠️  Failed to parse request for auto-read: %v", err)
		return
	}

	// Extract phone number from "to" field
	toField, ok := reqData["to"].(string)
	if !ok {
		log.Printf("⚠️  Missing 'to' field in request")
		return
	}

	// Clean phone number (remove @s.whatsapp.net suffix)
	phoneNumber := strings.TrimSuffix(toField, "@s.whatsapp.net")

	// 4. Get unread incoming messages for this contact
	unreadMessages, err := services.GetUnreadIncomingMessages(sessionToken, toField)
	if err != nil {
		log.Printf("⚠️  Failed to get unread messages: %v", err)
		return
	}

	if len(unreadMessages) == 0 {
		return // No unread messages, nothing to do
	}

	// 5. Extract message IDs
	messageIDs := make([]string, len(unreadMessages))
	for i, msg := range unreadMessages {
		messageIDs[i] = msg.MessageID
	}

	log.Printf("📖 Auto-reading %d unread messages for contact %s", len(messageIDs), phoneNumber)

	// 6. Call WA Server to mark messages as read
	if err := services.MarkMessagesAsRead(sessionToken, messageIDs, phoneNumber); err != nil {
		log.Printf("⚠️  Failed to mark messages as read via WA Server: %v", err)
		// Continue even if markread fails
	}

	// 7. Update database to mark messages as read
	if err := services.MarkMessagesAsReadInDB(messageIDs); err != nil {
		log.Printf("⚠️  Failed to mark messages as read in DB: %v", err)
	}
}

// handleSaveOutgoingMessage saves outgoing message to database after successful send
func handleSaveOutgoingMessage(sessionToken string, c *gin.Context, statusCode int) {
	// Parse request body to extract message details
	bodyBytes, err := c.GetRawData()
	if err != nil {
		log.Printf("⚠️  Failed to read request body for save: %v", err)
		return
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
		log.Printf("⚠️  Failed to parse request for save: %v", err)
		return
	}

	// Extract fields
	to, _ := reqData["to"].(string)
	var body string
	if text, ok := reqData["text"].(string); ok {
		body = text
	} else if caption, ok := reqData["caption"].(string); ok {
		body = caption
	}

	if to == "" || body == "" {
		return // Skip if no valid data
	}

	// Get session JID as "from"
	var session models.WhatsappSession
	if err := database.TransactionalDB.Where("token = ?", sessionToken).First(&session).Error; err != nil {
		log.Printf("⚠️  Failed to get session for save: %v", err)
		return
	}

	var from string
	if session.JID != nil && *session.JID != "" {
		from = *session.JID
	} else {
		from = sessionToken // Fallback to token if JID not available
	}

	// Generate message ID (or extract from response if available)
	messageID := fmt.Sprintf("%s_%d", sessionToken, time.Now().UnixNano())

	// Save to database with cleanup
	phoneNumber := strings.TrimSuffix(to, "@s.whatsapp.net")
	if err := services.SaveOutgoingMessageToAIChat(sessionToken, messageID, from, to, body, time.Now()); err != nil {
		log.Printf("⚠️  Failed to save outgoing message: %v", err)
		return
	}

	log.Printf("✅ Saved outgoing message to %s (contact: %s)", to, phoneNumber)
}
