package services

import (
	"fmt"
	"log"

	"genfity-wa-support/database"
	"genfity-wa-support/models"
)

// ContextData holds system prompt and user message for LLM
type ContextData struct {
	SystemPrompt string
	UserMessage  string
}

// Document represents knowledge base document
type Document struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Kind    string `json:"kind"`
}

// BotSettings holds bot configuration from transactional DB
type BotSettings struct {
	SystemPrompt string     `json:"systemPrompt"`
	FallbackText string     `json:"fallbackText"`
	Documents    []Document `json:"documents"`
}

// BuildContext fetches bot settings and builds context for LLM with default limit (10 messages)
func BuildContext(userID, sessionToken, messageID string) (*ContextData, error) {
	return BuildContextWithLimit(userID, sessionToken, messageID, 10)
}

// BuildContextWithLimit builds context with dynamic message limit
func BuildContextWithLimit(userID, sessionToken, messageID string, maxMessages int) (*ContextData, error) {
	// 1. Fetch bot settings using data provider (respects DATA_ACCESS_MODE env)
	provider, err := GetDataProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to get data provider: %w", err)
	}

	botSettings, err := provider.GetBotSettings(userID, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bot settings: %w", err)
	}

	// 2. Fetch chat history with dynamic limit
	db := database.GetDB()
	var messages []models.AIChatMessage
	err = db.Where("session_tok = ?", sessionToken).
		Order("timestamp DESC").
		Limit(maxMessages).
		Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chat history: %w", err)
	}

	// 3. Build system prompt
	systemPrompt := botSettings.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "Anda adalah customer service yang ramah dan profesional."
		log.Printf("⚠️  Using default system prompt (no custom prompt found)")
	} else {
		log.Printf("✅ Using custom system prompt: %d chars", len(systemPrompt))
		// Show first 300 chars of system prompt for debugging
		previewLen := min(300, len(systemPrompt))
		log.Printf("📝 System prompt preview: %s...", systemPrompt[:previewLen])
	}

	// Add knowledge base (limit to first 5 docs if too many)
	knowledgeLimit := 5
	if len(botSettings.Documents) > knowledgeLimit {
		log.Printf("⚠️  Limiting knowledge base to %d docs (total: %d)",
			knowledgeLimit, len(botSettings.Documents))
		botSettings.Documents = botSettings.Documents[:knowledgeLimit]
	}

	if len(botSettings.Documents) > 0 {
		systemPrompt += "\n\n=== Knowledge Base ===\n"
		for _, doc := range botSettings.Documents {
			// Limit doc content to 500 characters
			content := doc.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			systemPrompt += fmt.Sprintf("\n[%s - %s]\n%s\n", doc.Kind, doc.Title, content)
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
			// Limit message body to 200 characters
			body := msg.Body
			if len(body) > 200 {
				body = body[:200] + "..."
			}
			systemPrompt += fmt.Sprintf("%s: %s\n", role, body)
		}
	}

	// Get current message
	var currentMsg models.AIChatMessage
	err = db.Where("message_id = ?", messageID).First(&currentMsg).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current message: %w", err)
	}

	// Estimate token count (rough: 1 token ≈ 4 chars)
	estimatedTokens := (len(systemPrompt) + len(currentMsg.Body)) / 4
	log.Printf("📊 Context size: ~%d tokens (system: %d chars, user: %d chars, messages: %d)",
		estimatedTokens, len(systemPrompt), len(currentMsg.Body), maxMessages)

	return &ContextData{
		SystemPrompt: systemPrompt,
		UserMessage:  currentMsg.Body,
	}, nil
}
