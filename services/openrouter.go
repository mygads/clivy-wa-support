package services

import (
	"context"
	"fmt"
	"net/http"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

// NewOpenRouterClient creates OpenAI-compatible client for OpenRouter
func NewOpenRouterClient() *openai.Client {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		panic("OPENROUTER_API_KEY is required")
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

	return openai.NewClientWithConfig(cfg)
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
func AskLLM(ctx context.Context, client *openai.Client, systemPrompt, userMessage string) (string, int, int, error) {
	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = "openai/gpt-4o-mini"
	}

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
		Temperature: 0.3,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", 0, 0, err
	}

	if len(resp.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("no response from LLM")
	}

	output := resp.Choices[0].Message.Content
	inputTokens := resp.Usage.PromptTokens
	outputTokens := resp.Usage.CompletionTokens

	return output, inputTokens, outputTokens, nil
}
