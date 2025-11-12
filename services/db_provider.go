package services

import (
	"fmt"
	"log"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/models"

	"github.com/google/uuid"
)

// DBProvider implements DataProvider via direct DB access
type DBProvider struct {
	tablesVerified bool
}

// NewDBProvider creates new DB-based data provider
func NewDBProvider() (*DBProvider, error) {
	provider := &DBProvider{
		tablesVerified: false,
	}

	// Verify required tables exist (no auto-migrate!)
	if err := provider.verifyTablesExist(); err != nil {
		return nil, fmt.Errorf("❌ Direct DB mode failed: %w", err)
	}

	provider.tablesVerified = true
	log.Println("✅ Direct DB mode: All required tables exist")

	return provider, nil
}

// verifyTablesExist checks if all required Prisma tables exist
// Does NOT create tables - that's Prisma's job!
func (p *DBProvider) verifyTablesExist() error {
	db := database.GetTransactionalDB()
	if db == nil {
		return fmt.Errorf("transactional DB connection not initialized")
	}

	requiredTables := []string{
		"User",
		"WhatsAppSession",
		"ServicesWhatsappCustomers",
		"WhatsAppAIBot",
		"AIDocument",
		"AIUsageLog",
		"AIBotSessionBinding",
	}

	for _, table := range requiredTables {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf(
				"table '%s' not found. Please run Prisma migration first: npx prisma migrate deploy",
				table,
			)
		}
	}

	return nil
}

// ResolveSession resolves WhatsApp session token to user info via direct DB
func (p *DBProvider) ResolveSession(instanceName string) (*SessionInfo, error) {
	if !p.tablesVerified {
		return nil, fmt.Errorf("tables not verified")
	}

	db := database.GetTransactionalDB()

	// Query WhatsAppSession by token (instanceName)
	var session models.WhatsappSession
	if err := db.Where("token = ?", instanceName).First(&session).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	if session.UserID == nil {
		return nil, fmt.Errorf("session has no user")
	}

	// Check if bot is active for this session
	// Note: AIBotSessionBinding.sessionId refers to WhatsAppSession.id (not sessionId field)
	// Use quoted column names because Prisma uses camelCase
	var botBinding models.AIBotSessionBinding
	botActive := false

	log.Printf("🔍 Checking bot binding for session.ID: %s", session.ID)

	err := db.Where(`"sessionId" = ? AND "isActive" = ?`, session.ID, true).
		First(&botBinding).Error
	if err == nil {
		log.Printf("✓ Bot binding found: botID=%s", botBinding.BotID)
		// Binding exists, now check if bot is active
		var bot models.WhatsAppAIBot
		if err := db.Where(`"id" = ? AND "isActive" = ?`, botBinding.BotID, true).First(&bot).Error; err == nil {
			botActive = true
			log.Printf("✓ Bot is active: name=%s", bot.Name)
		} else {
			log.Printf("❌ Bot not active or not found: %v", err)
		}
	} else {
		log.Printf("❌ No bot binding found: %v", err)
	}

	// Check subscription status
	// Use quoted column names for Prisma camelCase columns
	var subscription models.ServicesWhatsappCustomers
	subscriptionActive := false

	log.Printf("🔍 Checking subscription for userID: %s", *session.UserID)

	err = db.Where(`"customerId" = ? AND "status" = ? AND "expiredAt" > ?`,
		*session.UserID, "active", time.Now()).
		First(&subscription).Error
	if err == nil {
		subscriptionActive = true
		log.Printf("✓ Subscription active: expires=%s", subscription.ExpiredAt)
	} else {
		log.Printf("❌ No active subscription found: %v", err)
	}

	return &SessionInfo{
		UserID:             *session.UserID,
		BotActive:          botActive,
		SubscriptionActive: subscriptionActive,
		SessionToken:       session.Token,
	}, nil
}

// GetBotSettings fetches AI bot configuration via direct DB
func (p *DBProvider) GetBotSettings(userID, sessionToken string) (*BotSettings, error) {
	if !p.tablesVerified {
		return nil, fmt.Errorf("tables not verified")
	}

	db := database.GetTransactionalDB()

	// Get bot by userId (use quoted column names for Prisma camelCase)
	var bot models.WhatsAppAIBot
	if err := db.Where(`"userId" = ? AND "isActive" = ?`, userID, true).First(&bot).Error; err != nil {
		return nil, fmt.Errorf("bot not found or inactive: %w", err)
	}

	// Get active documents (use quoted column names)
	var dbDocs []models.AIDocument
	if err := db.Where(`"userId" = ? AND "isActive" = ?`, userID, true).Find(&dbDocs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch documents: %w", err)
	}

	// Convert to Document slice
	documents := make([]Document, len(dbDocs))
	for i, doc := range dbDocs {
		documents[i] = Document{
			Title:   doc.Title,
			Content: doc.Content,
			Kind:    doc.Kind,
		}
	}

	systemPrompt := ""
	if bot.SystemPrompt != nil {
		systemPrompt = *bot.SystemPrompt
		log.Printf("✅ Bot settings loaded: botID=%s, promptLength=%d chars, docs=%d",
			bot.ID, len(systemPrompt), len(documents))
	} else {
		log.Printf("⚠️  No system prompt found for bot: %s", bot.ID)
	}

	fallbackText := ""
	if bot.FallbackText != nil {
		fallbackText = *bot.FallbackText
	}

	return &BotSettings{
		SystemPrompt: systemPrompt,
		FallbackText: fallbackText,
		Documents:    documents,
	}, nil
}

// LogUsage saves AI usage metrics via direct DB
func (p *DBProvider) LogUsage(logReq *UsageLogRequest) error {
	if !p.tablesVerified {
		return fmt.Errorf("tables not verified")
	}

	db := database.GetTransactionalDB()

	// Create usage log
	usageLog := models.AIUsageLog{
		ID:           uuid.New().String(),
		UserID:       logReq.UserID,
		SessionID:    nil,
		InputTokens:  logReq.InputTokens,
		OutputTokens: logReq.OutputTokens,
		TotalTokens:  logReq.TotalTokens,
		LatencyMs:    logReq.LatencyMs,
		Status:       logReq.Status,
		ErrorReason:  nil,
		CreatedAt:    time.Now(),
	}

	if logReq.SessionID != "" {
		usageLog.SessionID = &logReq.SessionID
	}

	if logReq.ErrorReason != "" {
		usageLog.ErrorReason = &logReq.ErrorReason
	}

	if err := db.Create(&usageLog).Error; err != nil {
		return fmt.Errorf("failed to save usage log: %w", err)
	}

	return nil
}

// CheckHealth verifies DB is ready
func (p *DBProvider) CheckHealth() error {
	if !p.tablesVerified {
		return fmt.Errorf("tables not verified")
	}

	db := database.GetTransactionalDB()
	if db == nil {
		return fmt.Errorf("transactional DB not connected")
	}

	// Try a simple query
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM \"User\"").Scan(&count).Error; err != nil {
		return fmt.Errorf("DB health check failed: %w", err)
	}

	return nil
}
