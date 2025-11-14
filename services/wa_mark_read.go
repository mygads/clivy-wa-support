package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// MarkReadRequest is the request body for WA Server /chat/markread
type MarkReadRequest struct {
	ID        []string `json:"Id"`        // Array of message IDs to mark as read
	ChatPhone string   `json:"ChatPhone"` // Phone number of the chat
}

// MarkMessagesAsRead calls WA Server API to mark messages as read
func MarkMessagesAsRead(sessionToken string, messageIDs []string, chatPhone string) error {
	if len(messageIDs) == 0 {
		return nil // Nothing to mark
	}

	waServerURL := os.Getenv("WHATSAPP_SERVER_API")
	if waServerURL == "" {
		return fmt.Errorf("WHATSAPP_SERVER_API not configured")
	}

	endpoint := fmt.Sprintf("%s/chat/markread", waServerURL)

	reqBody := MarkReadRequest{
		ID:        messageIDs,
		ChatPhone: chatPhone,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", sessionToken)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send markread request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ WA Server markread failed: status=%d, body=%s", resp.StatusCode, string(body))
		return fmt.Errorf("WA Server returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("✅ Marked %d messages as read for chat %s", len(messageIDs), chatPhone)
	return nil
}
