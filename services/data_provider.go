package services

import (
	"os"
)

// DataProvider is interface for accessing transactional data
// Can be implemented via API calls or direct DB access
type DataProvider interface {
	// ResolveSession resolves WhatsApp session token to user info
	ResolveSession(instanceName string) (*SessionInfo, error)

	// GetBotSettings fetches AI bot configuration and knowledge base
	GetBotSettings(userID, sessionToken string) (*BotSettings, error)

	// LogUsage saves AI usage metrics to transactional DB
	LogUsage(log *UsageLogRequest) error

	// CheckHealth verifies data provider is ready
	CheckHealth() error
}

// UsageLogRequest for logging AI usage
type UsageLogRequest struct {
	UserID       string
	SessionID    string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	LatencyMs    int
	Status       string
	ErrorReason  string
}

// GetDataProvider returns appropriate data provider based on env config
func GetDataProvider() (DataProvider, error) {
	mode := os.Getenv("DATA_ACCESS_MODE")
	if mode == "" {
		mode = "api" // Default to API mode (safer)
	}

	if mode == "direct" {
		return NewDBProvider()
	}

	// Default to API mode
	return NewAPIProvider(), nil
}
