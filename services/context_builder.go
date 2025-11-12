package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/models"
)

// ContextData holds system prompt and user message for LLM
type ContextData struct {
	SystemPrompt string
	UserMessage  string
}

// BotSettings holds bot configuration from transactional DB
type BotSettings struct {
	SystemPrompt string `json:"systemPrompt"`
	FallbackText string `json:"fallbackText"`
	Documents    []struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Kind    string `json:"kind"`
	} `json:"documents"`
}

// BuildContext fetches bot settings and builds context for LLM
func BuildContext(userID, sessionToken, messageID string) (*ContextData, error) {
	// 1. Fetch bot settings dari Transactional DB
	botSettings, err := fetchBotSettings(userID, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bot settings: %w", err)
	}

	// 2. Fetch chat history (last 10 messages)
	db := database.GetDB()
	var messages []models.AIChatMessage
	err = db.Where("session_tok = ?", sessionToken).
		Order("timestamp DESC").
		Limit(10).
		Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chat history: %w", err)
	}

	// 3. Build system prompt
	systemPrompt := botSettings.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "Anda adalah customer service yang ramah dan profesional."
	}

	// Add knowledge base
	if len(botSettings.Documents) > 0 {
		systemPrompt += "\n\n=== Knowledge Base ===\n"
		for _, doc := range botSettings.Documents {
			systemPrompt += fmt.Sprintf("\n[%s - %s]\n%s\n", doc.Kind, doc.Title, doc.Content)
		}
	}

	// Add chat history
	if len(messages) > 0 {
		systemPrompt += "\n\n=== Conversation History ===\n"
		// Reverse order (oldest first)
		for i := len(messages) - 1; i >= 0; i-- {
			msg := messages[i]
			role := "Customer"
			if msg.FromMe {
				role = "Assistant"
			}
			systemPrompt += fmt.Sprintf("%s: %s\n", role, msg.Body)
		}
	}

	// Get current message
	var currentMsg models.AIChatMessage
	err = db.Where("message_id = ?", messageID).First(&currentMsg).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current message: %w", err)
	}

	return &ContextData{
		SystemPrompt: systemPrompt,
		UserMessage:  currentMsg.Body,
	}, nil
}

// fetchBotSettings calls Transactional API to get bot configuration
func fetchBotSettings(userID, sessionToken string) (*BotSettings, error) {
	transactionalURL := os.Getenv("TRANSACTIONAL_API_URL")
	if transactionalURL == "" {
		transactionalURL = "http://localhost:8090/api"
	}

	url := fmt.Sprintf("%s/whatsapp/bot/settings?userId=%s&sessionToken=%s",
		transactionalURL, userID, sessionToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, body)
	}

	var settings BotSettings
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		return nil, err
	}

	return &settings, nil
}
