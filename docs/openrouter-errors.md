# OpenRouter Error Handling Guide

## Error Response Format

OpenRouter returns errors in a standardized JSON format:

```typescript
type ErrorResponse = {
  error: {
    code: number;
    message: string;
    metadata?: Record<string, unknown>;
  };
};
```

**Important**: HTTP status code matches `error.code` for request errors. Otherwise, status is `200 OK` with error details in response body.

## Common Error Codes

| HTTP Status | Code | Meaning | Solution |
|-------------|------|---------|----------|
| 400 | Bad Request | Invalid/missing parameters, CORS | Check request format |
| 401 | Unauthorized | Invalid API key | Verify `OPENROUTER_API_KEY` |
| 402 | Payment Required | Insufficient credits | Add credits |
| 403 | Forbidden | Input flagged by moderation | Review content policy |
| 408 | Request Timeout | Request timed out | Increase timeout or retry |
| 429 | Too Many Requests | Rate limited | Implement backoff |
| 502 | Bad Gateway | Model provider down | Retry or use different model |
| 503 | Service Unavailable | No available provider | Check model availability |

## Error Handling Implementation

### Go Implementation (Current)

```go
// services/openrouter.go
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
        // Error already contains HTTP status and message
        return "", 0, 0, fmt.Errorf("OpenRouter API error: %w", err)
    }

    if len(resp.Choices) == 0 {
        return "", 0, 0, fmt.Errorf("no response from LLM")
    }

    output := resp.Choices[0].Message.Content
    inputTokens := resp.Usage.PromptTokens
    outputTokens := resp.Usage.CompletionTokens

    return output, inputTokens, outputTokens, nil
}
```

### Worker Error Handling (Current)

```go
// services/ai_worker.go - Already implements retry logic
func processJob(db *gorm.DB, job *models.AIJob) error {
    // ... context building ...
    
    // Call LLM with timeout
    ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
    defer cancel()
    
    resp, inTok, outTok, err := AskLLM(ctx, openRouterClient, sysPrompt, userMsg)
    if err != nil {
        // Log error and mark job as failed
        job.Status = "failed"
        job.ErrorMsg = err.Error()
        job.UpdatedAt = time.Now()
        db.Save(job)
        
        // Log failed attempt
        logUsage(db, job, 0, 0, "error", err.Error())
        
        return err
    }
    
    // ... send reply ...
    
    return nil
}

// Retry logic in ProcessJobs (3 attempts, 30s interval)
if job.Attempts >= 3 {
    job.Status = "failed"
    db.Save(job)
    continue
}

job.Attempts++
job.NextRunAt = &nextRun  // 30 seconds from now
db.Save(job)
```

## Enhanced Error Handling (Recommended)

### 1. Parse Specific Error Types

```go
// services/openrouter_errors.go (NEW FILE)
package services

import (
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
)

type OpenRouterError struct {
    StatusCode int
    Code       int
    Message    string
    Metadata   map[string]interface{}
}

func (e *OpenRouterError) Error() string {
    return fmt.Sprintf("[OpenRouter %d] %s", e.Code, e.Message)
}

func ParseOpenRouterError(err error) (*OpenRouterError, bool) {
    // Try to parse as OpenRouter error
    var orErr *OpenRouterError
    if errors.As(err, &orErr) {
        return orErr, true
    }
    return nil, false
}

// Error type classification
func (e *OpenRouterError) IsRetryable() bool {
    return e.StatusCode == 408 || // Timeout
           e.StatusCode == 429 || // Rate limit
           e.StatusCode == 502 || // Bad gateway
           e.StatusCode == 503    // Service unavailable
}

func (e *OpenRouterError) IsAuthError() bool {
    return e.StatusCode == 401
}

func (e *OpenRouterError) IsPaymentError() bool {
    return e.StatusCode == 402
}

func (e *OpenRouterError) IsModerationError() bool {
    return e.StatusCode == 403
}
```

### 2. Add Exponential Backoff

```go
// services/ai_worker.go (ENHANCEMENT)
func processJobWithRetry(db *gorm.DB, job *models.AIJob) error {
    maxRetries := 3
    baseDelay := 5 * time.Second
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        err := processJob(db, job)
        
        if err == nil {
            return nil // Success!
        }
        
        // Parse error
        orErr, isORErr := ParseOpenRouterError(err)
        
        if isORErr {
            // Don't retry auth or payment errors
            if orErr.IsAuthError() || orErr.IsPaymentError() {
                return fmt.Errorf("non-retryable error: %w", err)
            }
            
            // Don't retry moderation errors
            if orErr.IsModerationError() {
                return fmt.Errorf("content moderation failed: %w", err)
            }
            
            // Only retry if error is retryable
            if !orErr.IsRetryable() && attempt < maxRetries-1 {
                continue // Try again without delay for non-retryable errors
            }
        }
        
        // Last attempt failed
        if attempt == maxRetries-1 {
            return err
        }
        
        // Exponential backoff
        delay := baseDelay * time.Duration(1<<uint(attempt))
        log.Printf("[Worker] Retry attempt %d after %v", attempt+1, delay)
        time.Sleep(delay)
    }
    
    return fmt.Errorf("max retries exceeded")
}
```

### 3. Handle Specific Errors

```go
// services/ai_worker.go (ENHANCEMENT)
func handleLLMError(db *gorm.DB, job *models.AIJob, err error) {
    orErr, isORErr := ParseOpenRouterError(err)
    
    if !isORErr {
        // Generic error
        job.Status = "failed"
        job.ErrorMsg = err.Error()
        db.Save(job)
        return
    }
    
    switch orErr.StatusCode {
    case 401:
        // Invalid API key - critical error
        log.Printf("[CRITICAL] Invalid OpenRouter API key!")
        job.Status = "failed"
        job.ErrorMsg = "Invalid API key"
        
    case 402:
        // Insufficient credits - needs user action
        log.Printf("[WARNING] Insufficient OpenRouter credits")
        job.Status = "failed"
        job.ErrorMsg = "Insufficient credits. Please add credits to OpenRouter."
        
    case 403:
        // Moderation flagged - content issue
        log.Printf("[MODERATION] Content flagged: %v", orErr.Metadata)
        job.Status = "failed"
        job.ErrorMsg = "Content violated moderation policy"
        
    case 408:
        // Timeout - retry
        log.Printf("[TIMEOUT] Request timed out, will retry")
        job.Status = "pending"
        nextRun := time.Now().Add(30 * time.Second)
        job.NextRunAt = &nextRun
        
    case 429:
        // Rate limited - backoff
        log.Printf("[RATE_LIMIT] Rate limited, backing off")
        job.Status = "pending"
        nextRun := time.Now().Add(60 * time.Second) // Wait 1 minute
        job.NextRunAt = &nextRun
        
    case 502, 503:
        // Provider down - retry with different model
        log.Printf("[PROVIDER_DOWN] Provider unavailable, will retry")
        job.Status = "pending"
        nextRun := time.Now().Add(30 * time.Second)
        job.NextRunAt = &nextRun
        
    default:
        // Unknown error
        job.Status = "failed"
        job.ErrorMsg = orErr.Message
    }
    
    db.Save(job)
}
```

## Moderation Errors

When content is flagged by moderation:

```typescript
type ModerationErrorMetadata = {
  reasons: string[];          // Why flagged
  flagged_input: string;      // Flagged text (max 100 chars)
  provider_name: string;      // Provider that requested moderation
  model_slug: string;         // Model identifier
};
```

**Example response:**
```json
{
  "error": {
    "code": 403,
    "message": "Your input was flagged",
    "metadata": {
      "reasons": ["violence", "hate"],
      "flagged_input": "inappropriate content...",
      "provider_name": "openai",
      "model_slug": "gpt-4"
    }
  }
}
```

**Handling strategy:**
1. Log flagged content for review
2. Notify user (generic message)
3. Don't retry automatically
4. Consider alternative phrasing

## Provider Errors

When model provider has issues:

```typescript
type ProviderErrorMetadata = {
  provider_name: string;  // Provider that failed
  raw: unknown;          // Raw error from provider
};
```

**Handling strategy:**
1. Log provider name
2. Implement automatic fallback to different provider
3. Retry after delay
4. Monitor provider reliability

## No Content Generated

Sometimes models return empty responses:

**Causes:**
- Model warming up (cold start)
- System scaling
- Provider issues

**Warm-up times:**
- Small models: few seconds
- Large models: few minutes

**Solution:**
```go
func AskLLM(...) (string, int, int, error) {
    // ... existing code ...
    
    if len(resp.Choices) == 0 {
        return "", 0, 0, fmt.Errorf("no response from LLM (possible cold start)")
    }
    
    output := resp.Choices[0].Message.Content
    
    // Check for empty content
    if output == "" {
        return "", resp.Usage.PromptTokens, resp.Usage.CompletionTokens, 
               fmt.Errorf("empty response from LLM")
    }
    
    return output, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, nil
}
```

**Note:** You may still be charged for prompt processing even if no content generated!

## Rate Limits

### Check API Key Status

```bash
curl https://openrouter.ai/api/v1/key \
  -H "Authorization: Bearer $OPENROUTER_API_KEY"
```

**Response:**
```json
{
  "data": {
    "label": "My API Key",
    "limit": null,              // Credit limit (null = unlimited)
    "limit_remaining": null,    // Remaining credits
    "usage": 0.05,             // All-time usage
    "usage_daily": 0.01,       // Today's usage
    "usage_weekly": 0.03,      // This week's usage
    "usage_monthly": 0.05,     // This month's usage
    "is_free_tier": false      // Has purchased credits
  }
}
```

### Free Model Limits

Models ending with `:free` have special limits:

- **60 requests/minute** (all users)
- **10 requests/day** (accounts with < $1 credits)
- **200 requests/day** (accounts with ≥ $1 credits)

### DDoS Protection

Cloudflare blocks excessive requests automatically.

### Monitoring Credits (Recommended)

```go
// services/credit_monitor.go (NEW FILE)
package services

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"
)

type CreditInfo struct {
    Data struct {
        Label          string  `json:"label"`
        Limit          *float64 `json:"limit"`
        LimitRemaining *float64 `json:"limit_remaining"`
        Usage          float64  `json:"usage"`
        UsageDaily     float64  `json:"usage_daily"`
        IsFreeTier     bool    `json:"is_free_tier"`
    } `json:"data"`
}

func CheckCredits() (*CreditInfo, error) {
    apiKey := os.Getenv("OPENROUTER_API_KEY")
    if apiKey == "" {
        return nil, fmt.Errorf("OPENROUTER_API_KEY not set")
    }
    
    req, _ := http.NewRequest("GET", "https://openrouter.ai/api/v1/key", nil)
    req.Header.Set("Authorization", "Bearer "+apiKey)
    
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var info CreditInfo
    if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
        return nil, err
    }
    
    return &info, nil
}

func MonitorCredits() {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    
    for range ticker.C {
        info, err := CheckCredits()
        if err != nil {
            log.Printf("[CreditMonitor] Error checking credits: %v", err)
            continue
        }
        
        if info.Data.LimitRemaining != nil && *info.Data.LimitRemaining < 1.0 {
            log.Printf("[CreditMonitor] WARNING: Low credits! Remaining: $%.2f", 
                      *info.Data.LimitRemaining)
        }
        
        log.Printf("[CreditMonitor] Daily usage: $%.4f", info.Data.UsageDaily)
    }
}
```

## Best Practices

### 1. Implement Circuit Breaker

```go
type CircuitBreaker struct {
    maxFailures int
    failures    int
    lastFailure time.Time
    cooldown    time.Duration
    isOpen      bool
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    // Check if circuit is open
    if cb.isOpen {
        if time.Since(cb.lastFailure) > cb.cooldown {
            cb.isOpen = false
            cb.failures = 0
        } else {
            return fmt.Errorf("circuit breaker open")
        }
    }
    
    err := fn()
    
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        
        if cb.failures >= cb.maxFailures {
            cb.isOpen = true
            log.Printf("[CircuitBreaker] Opened after %d failures", cb.failures)
        }
        
        return err
    }
    
    // Success - reset counter
    cb.failures = 0
    return nil
}
```

### 2. Log Errors Comprehensively

```go
func logLLMError(job *models.AIJob, err error) {
    log.Printf("[AI Error] JobID=%d, UserID=%s, SessionToken=%s, Error=%v",
        job.ID, job.UserID, job.SessionTok, err)
    
    // Also save to database for analytics
    db.Create(&ErrorLog{
        JobID:     job.ID,
        UserID:    job.UserID,
        ErrorType: "llm_error",
        Message:   err.Error(),
        CreatedAt: time.Now(),
    })
}
```

### 3. Set Appropriate Timeouts

```go
// services/openrouter.go
func AskLLM(...) {
    timeoutMs := getEnvInt("AI_TIMEOUT_MS", 120000)
    timeout := time.Duration(timeoutMs) * time.Millisecond
    
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    // Use context in request
    resp, err := client.CreateChatCompletion(ctx, req)
    // ...
}
```

### 4. Handle Context Exceeded Errors

```go
if len(resp.Choices) > 0 && resp.Choices[0].FinishReason == "length" {
    log.Printf("[Warning] Response truncated due to max_tokens limit")
    // Still return the partial response
    return resp.Choices[0].Message.Content, resp.Usage.PromptTokens, 
           resp.Usage.CompletionTokens, nil
}
```

## Testing Error Scenarios

### 1. Invalid API Key (401)
```bash
curl https://openrouter.ai/api/v1/chat/completions \
  -H "Authorization: Bearer invalid-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"Hi"}]}'
```

### 2. Insufficient Credits (402)
```bash
# Use valid key but with $0 balance
curl https://openrouter.ai/api/v1/chat/completions \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"openai/gpt-4o","messages":[{"role":"user","content":"Hi"}]}'
```

### 3. Invalid Request (400)
```bash
curl https://openrouter.ai/api/v1/chat/completions \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"invalid-model","messages":[]}'
```

## Summary

| Error Type | Retry? | Action |
|------------|--------|--------|
| 400 Bad Request | ❌ | Fix request format |
| 401 Unauthorized | ❌ | Check API key |
| 402 Payment Required | ❌ | Add credits |
| 403 Forbidden | ❌ | Review content |
| 408 Timeout | ✅ | Retry with backoff |
| 429 Rate Limited | ✅ | Retry after delay |
| 502 Bad Gateway | ✅ | Retry or fallback |
| 503 Service Unavailable | ✅ | Retry or fallback |

**Current Implementation:** ✅ Basic error handling with 3 retries
**Recommended:** ⏭️ Enhanced error parsing and circuit breaker

---

**See also:**
- [OpenRouter Configuration Guide](./openrouter-config.md)
- [OpenRouter Quick Start](./openrouter-quickstart.md)
