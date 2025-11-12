# AI Worker Quick Reference

## 🚀 Quick Start

### Environment Setup
```bash
# Required environment variables
OPENROUTER_API_KEY=sk-or-v1-...
OPENROUTER_MODEL=openai/gpt-4o
OPENROUTER_HTTP_REFERER=https://clivy.app
OPENROUTER_X_TITLE=Clivy
AI_TIMEOUT_MS=120000
```

### Start Server
```bash
go run main.go

# Should see:
# 🔍 Starting OpenRouter credit monitor...
# 🤖 AI Worker started
# 👂 Listening for AI job notifications...
# 🚀 Server starting on port 8070
```

### Stop Server (Gracefully)
```bash
# Press Ctrl+C

# Should see:
# 🛑 Shutting down server...
# 🤖 Stopping AI Worker...
# 🔕 Stopping job listener...
# ✅ AI Worker stopped
# ✅ Server exited gracefully
```

## 📊 Key Features

### 1. Circuit Breaker
**Purpose:** Prevent cascading failures when OpenRouter is down

**Config:**
- Max failures: 5
- Cooldown: 60 seconds

**States:**
- ✅ **Closed** - Normal operation
- ❌ **Open** - Provider failing, fast-fail all requests
- 🔄 **Half-Open** - Testing if provider recovered

**Logs:**
```
⚠️  Circuit breaker [openrouter] is OPEN (5 consecutive failures)
ℹ️  Circuit breaker [openrouter] entering Half-Open state (testing recovery)
✅ Circuit breaker [openrouter] is now Closed (recovered)
```

### 2. Smart Error Handling

**Error Types:**

| Type | HTTP Code | Action | Example |
|------|-----------|--------|---------|
| **Retryable** | 408, 429, 502, 503 | Retry up to 3x | Timeout, Rate limit, Gateway error |
| **Auth Error** | 401 | Permanent fail | Invalid API key |
| **Payment Error** | 402 | Permanent fail | Insufficient credits |
| **Moderation** | 403 | Permanent fail | Content flagged |
| **Context Length** | 400 | Retry with 5 msgs | Context too long |
| **Other** | 500 | Permanent fail | Unknown error |

**Logs:**
```
🔍 Analyzing error for job #123: context length exceeded
📏 Context too long, retrying job #123 with 5 messages instead of 10
✅ Job #123 completed with smaller context in 2500ms
```

### 3. Context Optimization

**Default:** 10 messages, 5 documents
**Fallback:** 5 messages (if context too long)

**Truncation:**
- Document content: 500 chars
- Message body: 200 chars

**Token Estimation:**
```
📊 Context size estimate: system=1234 chars, user=2345 chars, messages=10 (~897 tokens)
```

### 4. Credit Monitoring

**Frequency:** Every 1 hour

**Alerts:**
- 🔴 **CRITICAL:** Balance < $1
- 🟡 **WARNING:** Balance < $5
- 🟡 **High Usage:** Daily usage > $1

**Logs:**
```
🔍 Checking OpenRouter credits...
💰 OpenRouter Credits: $23.45 / $50.00 (46.9%)
   Usage today: $0.23 | Rate: $0.23/day
```

## 🛠️ Troubleshooting

### Problem: Circuit breaker constantly opening
**Symptoms:**
```
⚠️  Circuit breaker [openrouter] is OPEN (5 consecutive failures)
```

**Solutions:**
1. Check OpenRouter status: https://status.openrouter.ai
2. Verify API key: `echo $OPENROUTER_API_KEY`
3. Check credits: Monitor logs for balance
4. Increase cooldown: Modify `openRouterCB` in `ai_worker.go`

### Problem: Context too long errors
**Symptoms:**
```
📏 Context too long, retrying job #123 with 5 messages instead of 10
```

**Solutions:**
1. ✅ System automatically retries with 5 messages
2. If still failing, reduce truncation limits in `context_builder.go`:
   ```go
   const maxDocContent = 500  // Reduce to 300
   const maxMessageBody = 200 // Reduce to 100
   ```

### Problem: Credits running low
**Symptoms:**
```
🔴 CRITICAL: OpenRouter balance is $0.45 (below $1 threshold)
```

**Solutions:**
1. Add credits at: https://openrouter.ai/settings/credits
2. Set up auto-recharge
3. Monitor usage trends in logs

### Problem: Worker not processing jobs
**Symptoms:**
- Jobs stuck in `pending` status
- No log activity

**Solutions:**
1. Check database connection:
   ```bash
   psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT COUNT(*) FROM ai_jobs WHERE status='pending';"
   ```

2. Check LISTEN/NOTIFY:
   ```sql
   -- In psql
   LISTEN ai_jobs_channel;
   -- In another session:
   NOTIFY ai_jobs_channel, 'test';
   -- Should see notification
   ```

3. Restart worker:
   ```bash
   # Ctrl+C (graceful shutdown)
   # go run main.go
   ```

### Problem: Graceful shutdown not working
**Symptoms:**
- Server exits immediately on Ctrl+C
- "Server forced to shutdown" in logs

**Solutions:**
1. Check shutdown timeout (default 10s):
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
   ```

2. Increase timeout if processing long jobs:
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   ```

3. Check for deadlocks in worker goroutines

## 📈 Monitoring

### Key Metrics to Watch

**Job Processing:**
```bash
# Pending jobs
SELECT COUNT(*) FROM ai_jobs WHERE status='pending';

# Failed jobs (last hour)
SELECT COUNT(*) FROM ai_jobs 
WHERE status='failed' AND updated_at > NOW() - INTERVAL '1 hour';

# Average latency (last hour)
SELECT AVG((output_json::json->>'latency_ms')::int) 
FROM ai_jobs 
WHERE status='done' AND updated_at > NOW() - INTERVAL '1 hour';
```

**Token Usage:**
```bash
# Total tokens today
SELECT 
  SUM((output_json::json->>'input_tokens')::int) as input_tokens,
  SUM((output_json::json->>'output_tokens')::int) as output_tokens
FROM ai_jobs 
WHERE status='done' AND DATE(updated_at) = CURRENT_DATE;
```

**Error Rates:**
```bash
# Errors by type (last 24h)
SELECT error_msg, COUNT(*) 
FROM ai_jobs 
WHERE status='failed' 
  AND updated_at > NOW() - INTERVAL '24 hours'
GROUP BY error_msg 
ORDER BY COUNT(*) DESC 
LIMIT 10;
```

### Log Monitoring

**Success Patterns:**
```
✅ Job #123 completed in 2500ms (tokens: 1234 in, 567 out)
```

**Warning Patterns:**
```
⚠️  Circuit breaker [openrouter] is OPEN
🟡 WARNING: OpenRouter balance is $3.45 (below $5 threshold)
📏 Context too long, retrying with 5 messages
```

**Error Patterns:**
```
❌ Job #123 failed: LLM call failed (401): Authentication failed
🚫 Job #456 permanently failed: Non-retryable error: 402 - Insufficient credits
```

## 🔧 Configuration

### Circuit Breaker Tuning
**File:** `worker/ai_worker.go`

```go
// Adjust these values based on your needs:
var openRouterCB = services.NewCircuitBreaker(
    "openrouter",
    5,              // maxFailures - increase for more tolerance
    60*time.Second, // cooldown - increase for slower recovery attempts
)
```

**Recommendations:**
- **High traffic:** maxFailures=10, cooldown=120s
- **Low traffic:** maxFailures=3, cooldown=30s
- **Production:** maxFailures=5, cooldown=60s (default)

### Context Size Tuning
**File:** `services/context_builder.go`

```go
// Adjust truncation limits:
const maxDocContent = 500  // Characters per document
const maxMessageBody = 200 // Characters per message

// Adjust message limits in worker:
maxMessages := 10 // Default
maxMessages := 5  // Fallback
```

**Recommendations:**
- **GPT-4:** Keep defaults (10 messages, 500 char docs)
- **GPT-3.5:** Reduce to (8 messages, 400 char docs)
- **Claude:** Increase to (15 messages, 700 char docs)

### Credit Alert Thresholds
**File:** `services/credit_monitor.go`

```go
// Adjust alert thresholds:
if balance < 1.0 {
    // CRITICAL - change to 5.0 for earlier warning
}
if balance < 5.0 {
    // WARNING - change to 10.0 for more buffer
}
if usageToday > 1.0 {
    // High usage - change to 5.0 for higher threshold
}
```

### Retry Configuration
**File:** `worker/ai_worker.go` (failJob method)

```go
// Max retry attempts (currently 3)
if job.Attempts >= 3 {
    // Increase to 5 for more retries
}

// Retry interval (currently 30s)
nextRun := time.Now().Add(30 * time.Second)
// Increase to 60s for slower retries
```

## 🧪 Testing

### Test Circuit Breaker
```bash
# 1. Set invalid API key
export OPENROUTER_API_KEY=invalid

# 2. Send 5 messages
# Should see circuit open after 5th failure

# 3. Wait 60 seconds
# Circuit should enter half-open state

# 4. Set valid API key
export OPENROUTER_API_KEY=sk-or-v1-...

# 5. Send message
# Circuit should close on success
```

### Test Context Retry
```bash
# Send very long message (> 4000 chars)
curl -X POST http://localhost:8070/webhook/ai \
  -H "Content-Type: application/json" \
  -d '{
    "event": "Message",
    "instanceName": "test-session",
    "data": {
      "Info": {"ID": "msg123", "Sender": "6281234567890@s.whatsapp.net"},
      "Message": {"extendedTextMessage": {"text": "VERY LONG TEXT HERE..."}}
    }
  }'

# Should see:
# 📏 Context too long, retrying with 5 messages
# ✅ Job completed with smaller context
```

### Test Graceful Shutdown
```bash
# 1. Start server
go run main.go &
PID=$!

# 2. Send SIGTERM
kill -TERM $PID

# 3. Check logs
# Should see graceful shutdown messages

# 4. Verify no job loss
psql -d clivy_support -c "SELECT COUNT(*) FROM ai_jobs WHERE status='processing';"
# Should return 0
```

## 📚 Related Documentation

- [OpenRouter Quickstart](./openrouter-quickstart.md)
- [OpenRouter Errors](./openrouter-errors.md)
- [Code Improvements](./code-improvements.md)
- [Implementation Summary](./implementation-summary.md)
- [OpenRouter Responses API Beta](./openrouter-responses-api-beta.md)

## 🆘 Support

**Issues:** Check logs in `clivy-wa-support/logs/`
**Database:** PostgreSQL on localhost:5432
**OpenRouter Status:** https://status.openrouter.ai
**Documentation:** https://openrouter.ai/docs
