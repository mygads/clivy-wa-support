# Clivy AI Bot - Code Improvement Recommendations

**Last Updated**: November 12, 2025  
**Current Version**: v1.0 (Production-Ready)  
**Priority**: Optional Enhancements

## Overview

Implementasi saat ini sudah **production-ready** ✅, tapi ada beberapa improvement yang bisa meningkatkan reliability, performance, dan maintainability.

## Priority Levels

- 🔴 **Critical**: Harus dikerjakan sebelum production
- 🟡 **High**: Sangat disarankan untuk production
- 🟢 **Medium**: Nice to have, improve quality of life
- ⚪ **Low**: Optional, untuk future enhancement

---

## 1. Error Handling Enhancements

### 🟡 HIGH: Enhanced OpenRouter Error Parsing

**Current State:**
```go
// services/openrouter.go
resp, err := client.CreateChatCompletion(ctx, req)
if err != nil {
    return "", 0, 0, fmt.Errorf("OpenRouter API error: %w", err)
}
```

**Problem:**
- Error tidak dibedakan (retryable vs non-retryable)
- Tidak ada specific handling untuk auth, payment, moderation errors
- Worker retry semua error dengan logic sama

**Solution:**

Create `services/openrouter_errors.go`:

```go
package services

import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
)

type OpenRouterError struct {
    StatusCode int                    `json:"status_code"`
    Code       int                    `json:"code"`
    Message    string                 `json:"message"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

func (e *OpenRouterError) Error() string {
    return fmt.Sprintf("[OpenRouter %d] %s", e.Code, e.Message)
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

func (e *OpenRouterError) IsContextLengthError() bool {
    return e.StatusCode == 400 && 
           (e.Message == "Context length exceeded" || 
            e.Message == "Maximum context length exceeded")
}

// Parse error response
func ParseOpenRouterError(httpResp *http.Response) error {
    if httpResp.StatusCode >= 200 && httpResp.StatusCode < 300 {
        return nil
    }
    
    body, err := io.ReadAll(httpResp.Body)
    if err != nil {
        return fmt.Errorf("HTTP %d (failed to read body)", httpResp.StatusCode)
    }
    
    var errResp struct {
        Error struct {
            Code     int                    `json:"code"`
            Message  string                 `json:"message"`
            Metadata map[string]interface{} `json:"metadata"`
        } `json:"error"`
    }
    
    if err := json.Unmarshal(body, &errResp); err != nil {
        return fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(body))
    }
    
    return &OpenRouterError{
        StatusCode: httpResp.StatusCode,
        Code:       errResp.Error.Code,
        Message:    errResp.Error.Message,
        Metadata:   errResp.Error.Metadata,
    }
}
```

Update `worker/ai_worker.go`:

```go
func (w *AIWorker) processJob(job *models.AIJob) {
    // ... existing code ...
    
    response, inTok, outTok, err := services.AskLLM(timeoutCtx, w.llmClient, ctx.SystemPrompt, ctx.UserMessage)
    if err != nil {
        w.handleLLMError(job, &attempt, err)
        return
    }
    
    // ... rest of code ...
}

func (w *AIWorker) handleLLMError(job *models.AIJob, attempt *models.AIJobAttempt, err error) {
    var orErr *services.OpenRouterError
    
    if errors.As(err, &orErr) {
        // Handle specific errors
        switch {
        case orErr.IsAuthError():
            log.Printf("🔴 CRITICAL: Invalid OpenRouter API key!")
            w.permanentFailJob(job, attempt, "Invalid API key - check OPENROUTER_API_KEY")
            return
            
        case orErr.IsPaymentError():
            log.Printf("🔴 CRITICAL: Insufficient OpenRouter credits!")
            w.permanentFailJob(job, attempt, "Insufficient credits - add credits at openrouter.ai")
            return
            
        case orErr.IsModerationError():
            log.Printf("⚠️  Content moderation flagged: %v", orErr.Metadata)
            w.permanentFailJob(job, attempt, "Content violated moderation policy")
            return
            
        case orErr.IsContextLengthError():
            log.Printf("⚠️  Context too long, reducing history...")
            // Retry with shorter context (TODO: implement context reduction)
            w.failJob(job, attempt, "Context too long - will retry with reduced history")
            return
            
        case orErr.IsRetryable():
            log.Printf("⏰ Retryable error: %v", orErr.Message)
            w.failJob(job, attempt, orErr.Message) // Will retry
            return
        }
    }
    
    // Generic error
    w.failJob(job, attempt, err.Error())
}

func (w *AIWorker) permanentFailJob(job *models.AIJob, attempt *models.AIJobAttempt, errMsg string) {
    log.Printf("💀 Job #%d permanently failed: %s", job.ID, errMsg)
    
    now := time.Now()
    
    // Update attempt
    w.db.Model(attempt).Updates(map[string]interface{}{
        "status":    "error",
        "error_msg": errMsg,
        "ended_at":  now,
    })
    
    // Mark job as failed (no retry)
    w.db.Model(job).Updates(map[string]interface{}{
        "status":     "failed",
        "error_msg":  errMsg,
        "updated_at": now,
    })
    
    // Log to Transactional
    go w.logUsage(job.UserID, job.SessionTok, 0, 0, 0, "error", errMsg)
}
```

**Benefits:**
- ✅ Tidak retry error yang tidak perlu (auth, payment, moderation)
- ✅ Specific error messages untuk debugging
- ✅ Better logging dan monitoring
- ✅ User-friendly error messages

**Effort**: 2-3 hours  
**Impact**: High  
**Risk**: Low

---

## 2. Circuit Breaker Pattern

### 🟢 MEDIUM: Prevent Cascading Failures

**Current State:**
Worker terus-menerus call OpenRouter walaupun provider sedang down.

**Problem:**
- Waste resources pada request yang pasti gagal
- Increase latency untuk semua jobs
- Potential rate limiting

**Solution:**

Create `services/circuit_breaker.go`:

```go
package services

import (
    "fmt"
    "log"
    "sync"
    "time"
)

type CircuitBreaker struct {
    name         string
    maxFailures  int
    cooldown     time.Duration
    failures     int
    lastFailure  time.Time
    isOpen       bool
    mu           sync.RWMutex
}

func NewCircuitBreaker(name string, maxFailures int, cooldown time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        name:        name,
        maxFailures: maxFailures,
        cooldown:    cooldown,
    }
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    // Check if circuit is open
    if cb.isOpen {
        if time.Since(cb.lastFailure) > cb.cooldown {
            // Try half-open state
            cb.isOpen = false
            cb.failures = 0
            log.Printf("[CircuitBreaker:%s] Attempting half-open state", cb.name)
        } else {
            return fmt.Errorf("circuit breaker %s is open (cooldown until %v)", 
                cb.name, cb.lastFailure.Add(cb.cooldown))
        }
    }
    
    err := fn()
    
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        
        if cb.failures >= cb.maxFailures {
            cb.isOpen = true
            log.Printf("🔴 [CircuitBreaker:%s] OPENED after %d failures (cooldown: %v)", 
                cb.name, cb.failures, cb.cooldown)
        }
        
        return err
    }
    
    // Success - reset
    if cb.failures > 0 {
        log.Printf("✅ [CircuitBreaker:%s] Closed (recovered)", cb.name)
    }
    cb.failures = 0
    return nil
}

func (cb *CircuitBreaker) IsOpen() bool {
    cb.mu.RLock()
    defer cb.mu.RUnlock()
    return cb.isOpen
}
```

Use in worker:

```go
// worker/ai_worker.go

var openRouterCB = services.NewCircuitBreaker("openrouter", 5, 60*time.Second)

func (w *AIWorker) processJob(job *models.AIJob) {
    // ... context building ...
    
    // Check circuit breaker before calling
    if openRouterCB.IsOpen() {
        w.failJob(job, &attempt, "OpenRouter circuit breaker is open (provider may be down)")
        return
    }
    
    // Call LLM with circuit breaker
    var response string
    var inTok, outTok int
    
    err := openRouterCB.Call(func() error {
        var callErr error
        response, inTok, outTok, callErr = services.AskLLM(
            timeoutCtx, w.llmClient, ctx.SystemPrompt, ctx.UserMessage)
        return callErr
    })
    
    if err != nil {
        w.handleLLMError(job, &attempt, err)
        return
    }
    
    // ... rest of code ...
}
```

**Benefits:**
- ✅ Fast-fail saat provider down
- ✅ Automatic recovery setelah cooldown
- ✅ Reduce wasted resources
- ✅ Better error messages

**Effort**: 1-2 hours  
**Impact**: Medium  
**Risk**: Low

---

## 3. Context Length Optimization

### 🟢 MEDIUM: Dynamic Context Reduction

**Current State:**
```go
// services/context_builder.go
err = db.Where("session_tok = ?", sessionToken).
    Order("timestamp DESC").
    Limit(10).  // Fixed limit
    Find(&messages).Error
```

**Problem:**
- Context bisa exceed model limit (128K tokens untuk gpt-4o-mini)
- Tidak ada fallback saat context too long
- Fixed limit 10 messages tidak optimal

**Solution:**

Update `services/context_builder.go`:

```go
// BuildContextWithLimit builds context with dynamic message limit
func BuildContextWithLimit(userID, sessionToken, messageID string, maxMessages int) (*ContextData, error) {
    // ... existing bot settings fetch ...
    
    // Fetch chat history with dynamic limit
    db := database.GetDB()
    var messages []models.AIChatMessage
    err = db.Where("session_tok = ?", sessionToken).
        Order("timestamp DESC").
        Limit(maxMessages).
        Find(&messages).Error
    if err != nil {
        return nil, fmt.Errorf("failed to fetch chat history: %w", err)
    }
    
    // Build system prompt
    systemPrompt := botSettings.SystemPrompt
    if systemPrompt == "" {
        systemPrompt = "Anda adalah customer service yang ramah dan profesional."
    }
    
    // Add knowledge base (limit to first 5 docs if too many)
    knowledgeLimit := 5
    if len(botSettings.Documents) > knowledgeLimit {
        log.Printf("⚠️  Limiting knowledge base to %d docs (total: %d)", 
            knowledgeLimit, len(botSettings.Documents))
        botSettings.Documents = botSettings.Documents[:knowledgeLimit]
    }
    
    if len(botSettings.Documents) > 0 {
        systemPrompt += "\n\n=== Knowledge Base ===\n"
        for _, doc := range botSettings.Documents {
            // Limit doc content to 500 characters
            content := doc.Content
            if len(content) > 500 {
                content = content[:500] + "..."
            }
            systemPrompt += fmt.Sprintf("\n[%s - %s]\n%s\n", doc.Kind, doc.Title, content)
        }
    }
    
    // Add chat history
    if len(messages) > 0 {
        systemPrompt += "\n\n=== Conversation History ===\n"
        for i := len(messages) - 1; i >= 0; i-- {
            msg := messages[i]
            role := "Customer"
            if msg.FromMe {
                role = "Assistant"
            }
            // Limit message body to 200 characters
            body := msg.Body
            if len(body) > 200 {
                body = body[:200] + "..."
            }
            systemPrompt += fmt.Sprintf("%s: %s\n", role, body)
        }
    }
    
    // Get current message
    var currentMsg models.AIChatMessage
    err = db.Where("message_id = ?", messageID).First(&currentMsg).Error
    if err != nil {
        return nil, fmt.Errorf("failed to fetch current message: %w", err)
    }
    
    // Estimate token count (rough: 1 token ≈ 4 chars)
    estimatedTokens := (len(systemPrompt) + len(currentMsg.Body)) / 4
    log.Printf("📊 Context size: ~%d tokens (system: %d chars, user: %d chars)", 
        estimatedTokens, len(systemPrompt), len(currentMsg.Body))
    
    return &ContextData{
        SystemPrompt: systemPrompt,
        UserMessage:  currentMsg.Body,
    }, nil
}

// BuildContext uses default limit (10 messages)
func BuildContext(userID, sessionToken, messageID string) (*ContextData, error) {
    return BuildContextWithLimit(userID, sessionToken, messageID, 10)
}
```

Update worker untuk retry dengan context lebih pendek:

```go
// worker/ai_worker.go

func (w *AIWorker) processJob(job *models.AIJob) {
    // ... existing code ...
    
    // Build context with default limit (10 messages)
    ctx, err := services.BuildContext(job.UserID, job.SessionTok, job.MessageID)
    if err != nil {
        w.failJob(job, &attempt, fmt.Sprintf("Context build failed: %v", err))
        return
    }
    
    // Call LLM
    response, inTok, outTok, err := services.AskLLM(timeoutCtx, w.llmClient, ctx.SystemPrompt, ctx.UserMessage)
    if err != nil {
        // Check if context length error
        var orErr *services.OpenRouterError
        if errors.As(err, &orErr) && orErr.IsContextLengthError() {
            log.Printf("⚠️  Context too long, retrying with 5 messages...")
            
            // Retry dengan context lebih pendek
            ctx, err = services.BuildContextWithLimit(job.UserID, job.SessionTok, job.MessageID, 5)
            if err != nil {
                w.failJob(job, &attempt, fmt.Sprintf("Failed to build reduced context: %v", err))
                return
            }
            
            response, inTok, outTok, err = services.AskLLM(timeoutCtx, w.llmClient, ctx.SystemPrompt, ctx.UserMessage)
            if err != nil {
                w.handleLLMError(job, &attempt, err)
                return
            }
        } else {
            w.handleLLMError(job, &attempt, err)
            return
        }
    }
    
    // ... rest of code ...
}
```

**Benefits:**
- ✅ Handle context length errors gracefully
- ✅ Automatic fallback ke context lebih pendek
- ✅ Logging untuk debugging
- ✅ Reduce token costs

**Effort**: 2-3 hours  
**Impact**: Medium  
**Risk**: Low

---

## 4. Credit Monitoring

### 🟢 MEDIUM: Proactive Credit Alerts

**Current State:**
Tidak ada monitoring untuk OpenRouter credits. Baru tahu credit habis saat request gagal dengan 402.

**Solution:**

Create `services/credit_monitor.go`:

```go
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
        Label          string   `json:"label"`
        Limit          *float64 `json:"limit"`
        LimitRemaining *float64 `json:"limit_remaining"`
        Usage          float64  `json:"usage"`
        UsageDaily     float64  `json:"usage_daily"`
        UsageWeekly    float64  `json:"usage_weekly"`
        UsageMonthly   float64  `json:"usage_monthly"`
        IsFreeTier     bool     `json:"is_free_tier"`
    } `json:"data"`
}

func CheckCredits() (*CreditInfo, error) {
    apiKey := os.Getenv("OPENROUTER_API_KEY")
    if apiKey == "" {
        return nil, fmt.Errorf("OPENROUTER_API_KEY not set")
    }
    
    req, _ := http.NewRequest("GET", "https://openrouter.ai/api/v1/auth/key", nil)
    req.Header.Set("Authorization", "Bearer "+apiKey)
    
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("API returned %d", resp.StatusCode)
    }
    
    var info CreditInfo
    if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
        return nil, err
    }
    
    return &info, nil
}

func MonitorCredits() {
    // Initial check
    info, err := CheckCredits()
    if err != nil {
        log.Printf("⚠️  [CreditMonitor] Failed initial check: %v", err)
    } else {
        logCreditInfo(info)
    }
    
    // Check every hour
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    
    for range ticker.C {
        info, err := CheckCredits()
        if err != nil {
            log.Printf("⚠️  [CreditMonitor] Error: %v", err)
            continue
        }
        
        logCreditInfo(info)
        
        // Alert if low credits
        if info.Data.LimitRemaining != nil {
            remaining := *info.Data.LimitRemaining
            if remaining < 1.0 {
                log.Printf("🔴 [CreditMonitor] CRITICAL: Low credits! Remaining: $%.2f", remaining)
                // TODO: Send alert (email, Slack, etc.)
            } else if remaining < 5.0 {
                log.Printf("🟡 [CreditMonitor] WARNING: Credits running low. Remaining: $%.2f", remaining)
            }
        }
        
        // Alert if high daily usage
        if info.Data.UsageDaily > 1.0 {
            log.Printf("🟡 [CreditMonitor] High daily usage: $%.2f", info.Data.UsageDaily)
        }
    }
}

func logCreditInfo(info *CreditInfo) {
    if info.Data.LimitRemaining != nil {
        log.Printf("💰 [CreditMonitor] Remaining: $%.2f | Daily: $%.4f | Weekly: $%.4f | Monthly: $%.4f",
            *info.Data.LimitRemaining,
            info.Data.UsageDaily,
            info.Data.UsageWeekly,
            info.Data.UsageMonthly)
    } else {
        log.Printf("💰 [CreditMonitor] Daily: $%.4f | Weekly: $%.4f | Monthly: $%.4f | Total: $%.4f",
            info.Data.UsageDaily,
            info.Data.UsageWeekly,
            info.Data.UsageMonthly,
            info.Data.Usage)
    }
}
```

Start monitor in `main.go`:

```go
// main.go

func main() {
    // ... existing code ...
    
    // Start credit monitoring
    go services.MonitorCredits()
    
    // Start AI worker
    aiWorker := worker.NewAIWorker()
    go aiWorker.Start()
    
    // ... rest of code ...
}
```

**Benefits:**
- ✅ Proactive alerts sebelum credit habis
- ✅ Daily/weekly/monthly usage tracking
- ✅ Early warning system
- ✅ Better cost management

**Effort**: 1 hour  
**Impact**: Medium  
**Risk**: Very Low

---

## 5. Graceful Shutdown

### 🟡 HIGH: Prevent Job Loss on Restart

**Current State:**
Worker langsung stop saat service restart. Jobs yang sedang diproses bisa hilang atau corrupt.

**Solution:**

Update `worker/ai_worker.go`:

```go
type AIWorker struct {
    llmClient   *openai.Client
    db          *gorm.DB
    listener    *pq.Listener
    shutdown    chan struct{}
    wg          sync.WaitGroup
}

func NewAIWorker() *AIWorker {
    return &AIWorker{
        llmClient: services.NewOpenRouterClient(),
        db:        database.GetDB(),
        shutdown:  make(chan struct{}),
    }
}

func (w *AIWorker) Start() {
    log.Println("🤖 AI Worker started")
    
    // Setup LISTEN
    w.wg.Add(1)
    go w.listenForJobs()
    
    // Polling loop
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-w.shutdown:
            log.Println("🛑 AI Worker shutting down gracefully...")
            w.wg.Wait()
            log.Println("✅ AI Worker stopped")
            return
        case <-ticker.C:
            w.processJobs()
        }
    }
}

func (w *AIWorker) Stop() {
    close(w.shutdown)
}

func (w *AIWorker) listenForJobs() {
    defer w.wg.Done()
    
    // ... existing listener setup ...
    
    for {
        select {
        case <-w.shutdown:
            listener.Close()
            return
        case notification := <-listener.Notify:
            if notification != nil {
                log.Println("🔔 Job notification received")
                w.processJobs()
            }
        case <-time.After(90 * time.Second):
            go listener.Ping()
        }
    }
}
```

Update `main.go`:

```go
// main.go

import (
    "os"
    "os/signal"
    "syscall"
)

func main() {
    // ... existing setup ...
    
    // Start AI worker
    aiWorker := worker.NewAIWorker()
    go aiWorker.Start()
    
    // ... start HTTP server ...
    
    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    log.Println("🛑 Shutting down server...")
    
    // Stop AI worker first
    aiWorker.Stop()
    
    // Shutdown HTTP server
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Fatal("Server forced to shutdown:", err)
    }
    
    log.Println("✅ Server exited")
}
```

**Benefits:**
- ✅ No job loss saat restart
- ✅ Clean shutdown process
- ✅ Better production stability
- ✅ Graceful handling of SIGTERM

**Effort**: 2 hours  
**Impact**: High  
**Risk**: Low

---

## 6. Structured Logging

### 🟢 MEDIUM: Better Observability

**Current State:**
```go
log.Printf("⚙️  Processing job #%d", job.ID)
log.Printf("✅ Job #%d completed", job.ID)
```

**Problem:**
- Sulit parsing log untuk analytics
- Tidak ada structured fields (JSON)
- Sulit integrate dengan log aggregators (ELK, Datadog)

**Solution:**

Use `zerolog` atau `zap`:

```bash
go get github.com/rs/zerolog/log
```

Update logging:

```go
// worker/ai_worker.go

import (
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

func init() {
    // Pretty logging untuk development
    if os.Getenv("ENV") != "production" {
        log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
    }
}

func (w *AIWorker) processJob(job *models.AIJob) {
    logger := log.With().
        Str("component", "ai_worker").
        Uint("job_id", job.ID).
        Str("user_id", job.UserID).
        Str("session", job.SessionTok).
        Int("attempt", job.Attempts).
        Logger()
    
    logger.Info().Msg("Processing job")
    
    start := time.Now()
    
    // ... processing ...
    
    latency := time.Since(start).Milliseconds()
    
    logger.Info().
        Int("input_tokens", inTok).
        Int("output_tokens", outTok).
        Int64("latency_ms", latency).
        Str("status", "success").
        Msg("Job completed")
}

func (w *AIWorker) failJob(job *models.AIJob, attempt *models.AIJobAttempt, errMsg string) {
    logger := log.With().
        Str("component", "ai_worker").
        Uint("job_id", job.ID).
        Str("user_id", job.UserID).
        Int("attempt", job.Attempts).
        Logger()
    
    logger.Error().
        Str("error", errMsg).
        Bool("will_retry", job.Attempts < 3).
        Msg("Job failed")
    
    // ... existing code ...
}
```

**Benefits:**
- ✅ Structured logging (JSON)
- ✅ Easy to parse dan analyze
- ✅ Better debugging
- ✅ Ready untuk production monitoring

**Effort**: 2-3 hours  
**Impact**: Medium  
**Risk**: Low

---

## 7. Rate Limiting per Session

### ⚪ LOW: Prevent Abuse

**Current State:**
Tidak ada rate limiting. User bisa spam bot tanpa batas.

**Solution:**

Create `models/rate_limit.go`:

```go
type RateLimit struct {
    ID          uint      `gorm:"primaryKey"`
    SessionTok  string    `gorm:"index;not null"`
    WindowStart time.Time `gorm:"index;not null"`
    Count       int       `gorm:"default:0"`
    UpdatedAt   time.Time
}

func (RateLimit) TableName() string {
    return "rate_limits"
}
```

Create `services/rate_limiter.go`:

```go
package services

import (
    "fmt"
    "time"
    
    "genfity-wa-support/database"
    "genfity-wa-support/models"
)

const (
    RateLimitWindow  = 1 * time.Minute
    RateLimitMax     = 10 // 10 messages per minute
)

func CheckRateLimit(sessionToken string) error {
    db := database.GetDB()
    now := time.Now()
    windowStart := now.Add(-RateLimitWindow)
    
    // Get or create rate limit record
    var rl models.RateLimit
    err := db.Where("session_tok = ? AND window_start > ?", sessionToken, windowStart).
        First(&rl).Error
    
    if err != nil {
        // Create new window
        rl = models.RateLimit{
            SessionTok:  sessionToken,
            WindowStart: now,
            Count:       1,
            UpdatedAt:   now,
        }
        db.Create(&rl)
        return nil
    }
    
    // Check limit
    if rl.Count >= RateLimitMax {
        return fmt.Errorf("rate limit exceeded: %d messages in last minute", rl.Count)
    }
    
    // Increment counter
    db.Model(&rl).Updates(map[string]interface{}{
        "count":      rl.Count + 1,
        "updated_at": now,
    })
    
    return nil
}
```

Use in webhook:

```go
// handlers/ai_webhook.go

func HandleAIWebhook(c *gin.Context) {
    // ... existing parsing ...
    
    // Check rate limit
    if err := services.CheckRateLimit(sessionInfo.SessionToken); err != nil {
        log.Printf("⚠️  Rate limit exceeded for session %s", sessionInfo.SessionToken)
        c.JSON(http.StatusTooManyRequests, gin.H{
            "error": "Too many messages, please slow down"
        })
        return
    }
    
    // ... rest of code ...
}
```

**Benefits:**
- ✅ Prevent spam/abuse
- ✅ Control costs
- ✅ Better UX (prevent overload)
- ✅ Protect infrastructure

**Effort**: 2 hours  
**Impact**: Low  
**Risk**: Low

---

## 8. Response Caching

### ⚪ LOW: Reduce Costs for Common Questions

**Current State:**
Setiap pertanyaan selalu call LLM, even jika pertanyaan sama persis.

**Solution:**

Create `services/response_cache.go`:

```go
package services

import (
    "crypto/md5"
    "encoding/hex"
    "time"
    
    "genfity-wa-support/database"
)

type CachedResponse struct {
    ID           uint      `gorm:"primaryKey"`
    CacheKey     string    `gorm:"uniqueIndex;not null"`
    UserID       string    `gorm:"index;not null"`
    Question     string    `gorm:"type:text"`
    Response     string    `gorm:"type:text"`
    InputTokens  int
    OutputTokens int
    HitCount     int       `gorm:"default:0"`
    CreatedAt    time.Time
    ExpiresAt    time.Time `gorm:"index"`
}

func generateCacheKey(userID, question string) string {
    hash := md5.Sum([]byte(userID + "|" + question))
    return hex.EncodeToString(hash[:])
}

func GetCachedResponse(userID, question string) (*CachedResponse, bool) {
    db := database.GetDB()
    cacheKey := generateCacheKey(userID, question)
    
    var cached CachedResponse
    err := db.Where("cache_key = ? AND expires_at > ?", cacheKey, time.Now()).
        First(&cached).Error
    
    if err != nil {
        return nil, false
    }
    
    // Increment hit count
    db.Model(&cached).Updates(map[string]interface{}{
        "hit_count": cached.HitCount + 1,
    })
    
    return &cached, true
}

func SetCachedResponse(userID, question, response string, inputTokens, outputTokens int) {
    db := database.GetDB()
    cacheKey := generateCacheKey(userID, question)
    
    cached := CachedResponse{
        CacheKey:     cacheKey,
        UserID:       userID,
        Question:     question,
        Response:     response,
        InputTokens:  inputTokens,
        OutputTokens: outputTokens,
        HitCount:     0,
        CreatedAt:    time.Now(),
        ExpiresAt:    time.Now().Add(24 * time.Hour), // Cache 24 hours
    }
    
    db.Create(&cached)
}
```

Use in worker:

```go
// worker/ai_worker.go

func (w *AIWorker) processJob(job *models.AIJob) {
    // ... context building ...
    
    // Check cache first
    if cached, found := services.GetCachedResponse(job.UserID, ctx.UserMessage); found {
        log.Printf("💾 Cache hit for job #%d (saved %d tokens)", 
            job.ID, cached.InputTokens+cached.OutputTokens)
        
        // Send cached response
        services.SendWAText(job.SessionTok, chatMsg.From, cached.Response)
        
        // Mark job as done (with cached data)
        w.db.Model(job).Updates(map[string]interface{}{
            "status":     "done",
            "updated_at": time.Now(),
        })
        
        return
    }
    
    // Call LLM (cache miss)
    response, inTok, outTok, err := services.AskLLM(...)
    if err != nil {
        // ... error handling ...
    }
    
    // Cache the response
    go services.SetCachedResponse(job.UserID, ctx.UserMessage, response, inTok, outTok)
    
    // ... rest of code ...
}
```

**Benefits:**
- ✅ Reduce LLM costs (cache hits = $0)
- ✅ Faster responses
- ✅ Better UX
- ✅ Track common questions

**Effort**: 3-4 hours  
**Impact**: Low-Medium  
**Risk**: Low

---

## Summary Table

| Priority | Improvement | Effort | Impact | Risk | Status |
|----------|-------------|--------|--------|------|--------|
| 🟡 HIGH | Enhanced Error Parsing | 2-3h | High | Low | ⏭️ Recommended |
| 🟡 HIGH | Graceful Shutdown | 2h | High | Low | ⏭️ Recommended |
| 🟢 MEDIUM | Circuit Breaker | 1-2h | Medium | Low | ⏭️ Nice to have |
| 🟢 MEDIUM | Context Optimization | 2-3h | Medium | Low | ⏭️ Nice to have |
| 🟢 MEDIUM | Credit Monitoring | 1h | Medium | Very Low | ⏭️ Nice to have |
| 🟢 MEDIUM | Structured Logging | 2-3h | Medium | Low | ⏭️ Nice to have |
| ⚪ LOW | Rate Limiting | 2h | Low | Low | ⏸️ Optional |
| ⚪ LOW | Response Caching | 3-4h | Low-Med | Low | ⏸️ Optional |

## Implementation Order

### Phase 1: Critical (Before Production)
1. ✅ **DONE**: Current implementation already production-ready

### Phase 2: High Priority (Week 1)
1. 🟡 Enhanced Error Parsing
2. 🟡 Graceful Shutdown
3. 🟢 Credit Monitoring

**Total effort**: ~5-6 hours

### Phase 3: Nice to Have (Week 2-3)
1. 🟢 Circuit Breaker
2. 🟢 Context Optimization
3. 🟢 Structured Logging

**Total effort**: ~5-8 hours

### Phase 4: Optional (Future)
1. ⚪ Rate Limiting
2. ⚪ Response Caching

**Total effort**: ~5-6 hours

## Conclusion

**Current state**: ✅ Production-ready  
**Recommended improvements**: 🟡 HIGH priority items (5-6 hours work)  
**Optional enhancements**: 🟢 MEDIUM + ⚪ LOW (10-14 hours total)

**Recommendation**: Ship to production NOW, implement HIGH priority improvements in Week 1, monitor dan evaluate sebelum implement optional features.

---

**See also:**
- [OpenRouter Configuration Guide](./openrouter-config.md)
- [OpenRouter Error Handling](./openrouter-errors.md)
- [OpenRouter Responses API Beta](./openrouter-responses-api-beta.md)
