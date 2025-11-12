package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// OpenRouterError represents a structured error from OpenRouter API
type OpenRouterError struct {
	StatusCode int                    `json:"status_code"`
	Code       int                    `json:"code"`
	Message    string                 `json:"message"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

func (e *OpenRouterError) Error() string {
	return fmt.Sprintf("[OpenRouter %d] %s", e.Code, e.Message)
}

// Error type classification methods

// IsRetryable returns true if the error is temporary and can be retried
func (e *OpenRouterError) IsRetryable() bool {
	return e.StatusCode == 408 || // Request Timeout
		e.StatusCode == 429 || // Too Many Requests
		e.StatusCode == 502 || // Bad Gateway
		e.StatusCode == 503 // Service Unavailable
}

// IsAuthError returns true if the error is related to authentication
func (e *OpenRouterError) IsAuthError() bool {
	return e.StatusCode == 401
}

// IsPaymentError returns true if the error is related to insufficient credits
func (e *OpenRouterError) IsPaymentError() bool {
	return e.StatusCode == 402
}

// IsModerationError returns true if the content was flagged by moderation
func (e *OpenRouterError) IsModerationError() bool {
	return e.StatusCode == 403
}

// IsContextLengthError returns true if the context is too long
func (e *OpenRouterError) IsContextLengthError() bool {
	if e.StatusCode != 400 {
		return false
	}

	msgLower := strings.ToLower(e.Message)
	return strings.Contains(msgLower, "context") &&
		(strings.Contains(msgLower, "length") ||
			strings.Contains(msgLower, "exceeded") ||
			strings.Contains(msgLower, "too long"))
}

// ParseOpenRouterError parses HTTP response into OpenRouterError
func ParseOpenRouterError(httpResp *http.Response) error {
	if httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {
		return nil
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("HTTP %d (failed to read body)", httpResp.StatusCode)
	}

	// Try to parse as OpenRouter error format
	var errResp struct {
		Error struct {
			Code     int                    `json:"code"`
			Message  string                 `json:"message"`
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		// Not a JSON error response, return raw body
		return fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(body))
	}

	return &OpenRouterError{
		StatusCode: httpResp.StatusCode,
		Code:       errResp.Error.Code,
		Message:    errResp.Error.Message,
		Metadata:   errResp.Error.Metadata,
	}
}

// ParseSDKError converts go-openai SDK error to OpenRouterError
func ParseSDKError(err error) *OpenRouterError {
	if err == nil {
		return nil
	}

	// Try to unwrap as OpenAI API error
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return &OpenRouterError{
			StatusCode: apiErr.HTTPStatusCode,
			Code:       apiErr.HTTPStatusCode,
			Message:    apiErr.Message,
		}
	}

	// Fallback: parse error message for common patterns
	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	// Check for timeout
	if strings.Contains(errMsgLower, "timeout") || strings.Contains(errMsgLower, "deadline exceeded") {
		return &OpenRouterError{
			StatusCode: 408,
			Code:       408,
			Message:    "Request timeout",
		}
	}

	// Check for context length
	if strings.Contains(errMsgLower, "context") &&
		(strings.Contains(errMsgLower, "length") || strings.Contains(errMsgLower, "too long")) {
		return &OpenRouterError{
			StatusCode: 400,
			Code:       400,
			Message:    errMsg,
		}
	}

	// Check for auth errors
	if strings.Contains(errMsgLower, "unauthorized") || strings.Contains(errMsgLower, "invalid api key") {
		return &OpenRouterError{
			StatusCode: 401,
			Code:       401,
			Message:    "Authentication failed",
		}
	}

	// Check for payment errors
	if strings.Contains(errMsgLower, "insufficient") || strings.Contains(errMsgLower, "quota") || strings.Contains(errMsgLower, "billing") {
		return &OpenRouterError{
			StatusCode: 402,
			Code:       402,
			Message:    "Insufficient credits or quota exceeded",
		}
	}

	// Check for rate limiting
	if strings.Contains(errMsgLower, "rate limit") || strings.Contains(errMsgLower, "too many requests") {
		return &OpenRouterError{
			StatusCode: 429,
			Code:       429,
			Message:    "Rate limit exceeded",
		}
	}

	// Check for server errors
	if strings.Contains(errMsgLower, "bad gateway") {
		return &OpenRouterError{
			StatusCode: 502,
			Code:       502,
			Message:    "Bad gateway",
		}
	}

	if strings.Contains(errMsgLower, "service unavailable") || strings.Contains(errMsgLower, "temporarily unavailable") {
		return &OpenRouterError{
			StatusCode: 503,
			Code:       503,
			Message:    "Service temporarily unavailable",
		}
	}

	// Unknown error - treat as non-retryable
	return &OpenRouterError{
		StatusCode: 500,
		Code:       500,
		Message:    errMsg,
	}
}
