package services

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// OpenRouterClient wraps OpenAI-compatible client for OpenRouter
type OpenRouterClient struct {
	client  *openai.Client
	model   string
	timeout time.Duration
}

// NewOpenRouterClient creates OpenAI-compatible client for OpenRouter
func NewOpenRouterClient() (*OpenRouterClient, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY not set in environment")
	}

	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = "openai/gpt-4o-mini" // default model
	}

	timeoutMs := 120000 // default 120 seconds
	if t := os.Getenv("AI_TIMEOUT_MS"); t != "" {
		if parsed, err := strconv.Atoi(t); err == nil {
			timeoutMs = parsed
		}
	}

	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = "https://openrouter.ai/api/v1"

	// Add custom headers for OpenRouter
	referer := os.Getenv("OPENROUTER_HTTP_REFERER")
	if referer == "" {
		referer = "https://clivy.app"
	}

	title := os.Getenv("OPENROUTER_X_TITLE")
	if title == "" {
		title = "Clivy"
	}

	cfg.HTTPClient = &http.Client{
		Transport: &openRouterTransport{
			base:    http.DefaultTransport,
			referer: referer,
			title:   title,
		},
	}

	client := openai.NewClientWithConfig(cfg)

	log.Printf("[OpenRouterClient] Initialized with model=%s, timeout=%dms", model, timeoutMs)

	return &OpenRouterClient{
		client:  client,
		model:   model,
		timeout: time.Duration(timeoutMs) * time.Millisecond,
	}, nil
}

// openRouterTransport adds custom headers
type openRouterTransport struct {
	base    http.RoundTripper
	referer string
	title   string
}

func (t *openRouterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("HTTP-Referer", t.referer)
	req.Header.Set("X-Title", t.title)
	return t.base.RoundTrip(req)
}

// AskLLM sends prompt to LLM and returns response with token counts
func (orc *OpenRouterClient) AskLLM(ctx context.Context, systemPrompt, userMessage string) (string, int, int, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, orc.timeout)
	defer cancel()

	startTime := time.Now()

	req := openai.ChatCompletionRequest{
		Model: orc.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
		Temperature: 0.3,
	}

	resp, err := orc.client.CreateChatCompletion(timeoutCtx, req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("OpenRouter API error: %w", err)
	}

	latency := time.Since(startTime).Milliseconds()

	if len(resp.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("no response from LLM")
	}

	output := resp.Choices[0].Message.Content
	inputTokens := resp.Usage.PromptTokens
	outputTokens := resp.Usage.CompletionTokens

	log.Printf("[OpenRouterClient] Success | model=%s | latency=%dms | in=%d | out=%d | total=%d",
		orc.model, latency, inputTokens, outputTokens, inputTokens+outputTokens)

	return output, inputTokens, outputTokens, nil
}

// GetProviderName returns the provider name for logging
func (orc *OpenRouterClient) GetProviderName() string {
	return "openrouter"
}

// GetModelName returns the model name being used
func (orc *OpenRouterClient) GetModelName() string {
	return orc.model
}
