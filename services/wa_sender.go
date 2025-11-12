package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SendTextRequest payload for WA Service
type SendTextRequest struct {
	SessionID string `json:"sessionId"`
	To        string `json:"to"`
	Text      string `json:"text"`
}

// SendWAText sends text message via internal Gateway (reuses existing validation & tracking)
// Gateway akan handle:
// - Validasi token & subscription
// - Track message stats ke DB Transactional
// - Proxy ke WA Server (port 8080)
func SendWAText(sessionToken, to, text string) error {
	// Call internal gateway endpoint (localhost:8070/wa/chat/send/text)
	// Gateway sudah handle semua validasi dan tracking
	url := "http://localhost:8070/wa/chat/send/text"

	payload := SendTextRequest{
		SessionID: sessionToken,
		To:        to,
		Text:      text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers (gateway needs token for validation)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", sessionToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send WA message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway returned %d", resp.StatusCode)
	}

	return nil
}
