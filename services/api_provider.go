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

// APIProvider implements DataProvider via HTTP API calls to Next.js
type APIProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewAPIProvider creates new API-based data provider
func NewAPIProvider() *APIProvider {
	transactionalURL := os.Getenv("TRANSACTIONAL_API_URL")
	if transactionalURL == "" {
		transactionalURL = "http://localhost:8090/api"
	}

	apiKey := os.Getenv("INTERNAL_API_KEY")

	return &APIProvider{
		baseURL: transactionalURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ResolveSession resolves WhatsApp session token to user info via API
func (p *APIProvider) ResolveSession(instanceName string) (*SessionInfo, error) {
	url := fmt.Sprintf("%s/whatsapp/session/resolve?token=%s", p.baseURL, instanceName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Debug logging
	log.Printf("🔑 Making request to: %s", url)
	log.Printf("🔑 API Key configured: %v (length: %d)", p.apiKey != "", len(p.apiKey))
	if p.apiKey != "" {
		log.Printf("🔑 API Key preview: %s...", p.apiKey[:10])
	}

	// Add internal API key if configured
	if p.apiKey != "" {
		req.Header.Set("x-api-key", p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call session resolve API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("session resolve API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool        `json:"success"`
		Data    SessionInfo `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API returned success=false")
	}

	return &result.Data, nil
}

// GetBotSettings fetches AI bot configuration via API
func (p *APIProvider) GetBotSettings(userID, sessionToken string) (*BotSettings, error) {
	url := fmt.Sprintf("%s/whatsapp/bot/settings?userId=%s&sessionToken=%s",
		p.baseURL, userID, sessionToken)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add internal API key if configured
	if p.apiKey != "" {
		req.Header.Set("x-api-key", p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call bot settings API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bot settings API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success bool        `json:"success"`
		Data    BotSettings `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API returned success=false")
	}

	return &result.Data, nil
}

// LogUsage saves AI usage metrics via API
func (p *APIProvider) LogUsage(log *UsageLogRequest) error {
	url := fmt.Sprintf("%s/customer/ai/usage", p.baseURL)

	payload := map[string]interface{}{
		"userId":       log.UserID,
		"sessionId":    log.SessionID,
		"inputTokens":  log.InputTokens,
		"outputTokens": log.OutputTokens,
		"totalTokens":  log.TotalTokens,
		"latencyMs":    log.LatencyMs,
		"status":       log.Status,
		"errorReason":  log.ErrorReason,
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("x-api-key", p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call usage log API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("usage log API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CheckHealth verifies API is reachable
func (p *APIProvider) CheckHealth() error {
	// Try to ping a simple endpoint (session resolve with dummy token)
	// Just to check if API is up
	resp, err := p.client.Get(p.baseURL + "/whatsapp/session/resolve?token=healthcheck")
	if err != nil {
		return fmt.Errorf("API not reachable: %w", err)
	}
	defer resp.Body.Close()

	// We expect 404 or 400 (token not found), not connection error
	// Any HTTP response means API is up
	return nil
}
