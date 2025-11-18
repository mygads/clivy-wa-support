package services

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// GetAIProvider creates and returns the appropriate AI provider based on configuration
func GetAIProvider() (AIProvider, error) {
	providerMode := strings.ToLower(os.Getenv("AI_PROVIDER"))

	// Default to openrouter if not specified
	if providerMode == "" {
		providerMode = "openrouter"
		log.Printf("[AIProvider] AI_PROVIDER not set, defaulting to 'openrouter'")
	}

	log.Printf("[AIProvider] Initializing AI provider: %s", providerMode)

	switch providerMode {
	case "openrouter":
		client, err := NewOpenRouterClient()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize OpenRouter: %w", err)
		}
		log.Printf("[AIProvider] ✓ OpenRouter client ready (model: %s)", client.GetModelName())
		return client, nil

	case "gemini":
		client, err := NewGeminiClient()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Gemini: %w", err)
		}
		log.Printf("[AIProvider] ✓ Gemini client ready (model: %s)", client.GetModelName())
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported AI_PROVIDER: %s (valid options: openrouter, gemini)", providerMode)
	}
}
