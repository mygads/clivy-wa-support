package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// TypingStateRequest represents the payload for typing indicator
type TypingStateRequest struct {
	Phone string `json:"Phone"`
	State string `json:"State"` // "composing" or "stop"
}

// SetTypingState sends typing indicator to WhatsApp Server
// state can be "composing" (start typing) or "stop" (stop typing)
func SetTypingState(sessionToken, phone, state string) error {
	waServerAPI := os.Getenv("WHATSAPP_SERVER_API")
	if waServerAPI == "" {
		return fmt.Errorf("WHATSAPP_SERVER_API not configured")
	}

	url := fmt.Sprintf("%s/chat/presence", waServerAPI)

	payload := TypingStateRequest{
		Phone: phone,
		State: state,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal typing state payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create typing state request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", sessionToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send typing state request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("typing state request failed with status %d", resp.StatusCode)
	}

	return nil
}
