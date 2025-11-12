package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// SessionInfo holds user and bot configuration from transactional DB
type SessionInfo struct {
	UserID             string `json:"userId"`
	BotActive          bool   `json:"botActive"`
	SubscriptionActive bool   `json:"subscriptionActive"`
	SessionToken       string `json:"sessionToken"`
}

// ResolveSession calls Transactional API to get user and bot info
func ResolveSession(sessionToken string) (*SessionInfo, error) {
	transactionalURL := os.Getenv("TRANSACTIONAL_API_URL")
	if transactionalURL == "" {
		transactionalURL = "http://localhost:8090/api"
	}

	url := fmt.Sprintf("%s/whatsapp/session/resolve?token=%s", transactionalURL, sessionToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call transactional API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, body)
	}

	var info SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &info, nil
}
