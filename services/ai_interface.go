package services

import "context"

// AIProvider is the interface that all AI providers must implement
type AIProvider interface {
	// AskLLM sends a prompt to the AI and returns response with token usage
	// Returns: (response string, inputTokens int, outputTokens int, error)
	AskLLM(ctx context.Context, systemPrompt string, userPrompt string) (string, int, int, error)

	// GetProviderName returns the name of the provider (e.g., "openrouter", "gemini")
	GetProviderName() string

	// GetModelName returns the model name being used
	GetModelName() string
}
