package services

import (
	"log"
)

// SessionInfo holds user and bot configuration from transactional DB
type SessionInfo struct {
	UserID             string `json:"userId"`
	BotActive          bool   `json:"botActive"`
	SubscriptionActive bool   `json:"subscriptionActive"`
	SessionToken       string `json:"sessionToken"`
}

// Global data provider instance
var dataProvider DataProvider

// InitDataProvider initializes the data provider based on config
func InitDataProvider() error {
	provider, err := GetDataProvider()
	if err != nil {
		return err
	}
	dataProvider = provider
	log.Printf("✅ Data provider initialized in mode: %T", dataProvider)
	return nil
}

// ResolveSession calls appropriate data provider to get user and bot info
func ResolveSession(sessionToken string) (*SessionInfo, error) {
	if dataProvider == nil {
		// Fallback: initialize if not done yet
		if err := InitDataProvider(); err != nil {
			return nil, err
		}
	}

	return dataProvider.ResolveSession(sessionToken)
}
