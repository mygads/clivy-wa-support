package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"google.golang.org/genai"
)

// GeminiClient wraps Google Gemini API client
type GeminiClient struct {
	client  *genai.Client
	model   string
	timeout time.Duration
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient() (*GeminiClient, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY not set in environment")
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash" // default model
	}

	timeoutMs := 120000 // default 120 seconds
	if t := os.Getenv("AI_TIMEOUT_MS"); t != "" {
		if parsed, err := strconv.Atoi(t); err == nil {
			timeoutMs = parsed
		}
	}

	ctx := context.Background()

	// Initialize Gemini client with API key
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	log.Printf("[GeminiClient] Initialized with model=%s, timeout=%dms", model, timeoutMs)

	return &GeminiClient{
		client:  client,
		model:   model,
		timeout: time.Duration(timeoutMs) * time.Millisecond,
	}, nil
}

// AskLLM sends a prompt to Gemini and returns the response with token usage
func (gc *GeminiClient) AskLLM(ctx context.Context, systemPrompt string, userPrompt string) (string, int, int, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, gc.timeout)
	defer cancel()

	// Combine system and user prompts
	// Gemini doesn't have explicit system role, so we prepend it to the user message
	fullPrompt := systemPrompt + "\n\n" + userPrompt

	startTime := time.Now()

	// Generate content
	result, err := gc.client.Models.GenerateContent(
		timeoutCtx,
		gc.model,
		genai.Text(fullPrompt),
		nil,
	)
	if err != nil {
		return "", 0, 0, fmt.Errorf("Gemini API error: %w", err)
	}

	latency := time.Since(startTime).Milliseconds()

	// Extract response text
	responseText := ""
	if result != nil && len(result.Candidates) > 0 {
		responseText = result.Text()
	}

	if responseText == "" {
		return "", 0, 0, fmt.Errorf("empty response from Gemini")
	}

	// Extract token usage
	inputTokens := 0
	outputTokens := 0

	if result.UsageMetadata != nil {
		inputTokens = int(result.UsageMetadata.PromptTokenCount)
		outputTokens = int(result.UsageMetadata.CandidatesTokenCount)
	}

	log.Printf("[GeminiClient] Success | model=%s | latency=%dms | in=%d | out=%d | total=%d",
		gc.model, latency, inputTokens, outputTokens, inputTokens+outputTokens)

	return responseText, inputTokens, outputTokens, nil
}

// GetProviderName returns the provider name for logging
func (gc *GeminiClient) GetProviderName() string {
	return "gemini"
}

// GetModelName returns the model name being used
func (gc *GeminiClient) GetModelName() string {
	return gc.model
}
