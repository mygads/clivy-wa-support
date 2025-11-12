# OpenRouter Configuration Guide

## Overview

Clivy AI Bot menggunakan **OpenRouter** sebagai gateway untuk mengakses berbagai LLM providers (OpenAI, Anthropic, Meta, Google, dll) dengan satu API yang konsisten.

## Mengapa OpenRouter?

✅ **Multi-Provider Access**: Akses ke 100+ model dari berbagai provider  
✅ **Unified API**: Compatible dengan OpenAI SDK  
✅ **Cost Optimization**: Automatic failover dan routing  
✅ **No Rate Limits**: Pay-per-use tanpa rate limiting  
✅ **Transparent Pricing**: Token-based billing yang jelas

## Environment Variables

### Required Variables

```bash
# API Key dari https://openrouter.ai/keys
OPENROUTER_API_KEY=sk-or-v1-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# Model yang akan digunakan (dengan organization prefix)
OPENROUTER_MODEL=openai/gpt-4o-mini

# Timeout untuk request LLM (dalam milliseconds)
AI_TIMEOUT_MS=120000
```

### Optional Variables

```bash
# HTTP Referer - Identifikasi aplikasi Anda di openrouter.ai
OPENROUTER_HTTP_REFERER=https://clivy.app

# X-Title - Nama aplikasi untuk ranking di openrouter.ai
OPENROUTER_X_TITLE=Clivy

# Temperature (0.0 - 2.0, default: 0.3)
OPENROUTER_TEMPERATURE=0.3

# Max Tokens (default: auto dari model context)
OPENROUTER_MAX_TOKENS=1000

# Top P (0.0 - 1.0, default: 1.0)
OPENROUTER_TOP_P=1.0

# Frequency Penalty (-2.0 - 2.0, default: 0.0)
OPENROUTER_FREQUENCY_PENALTY=0.0

# Presence Penalty (-2.0 - 2.0, default: 0.0)
OPENROUTER_PRESENCE_PENALTY=0.0
```

## Available Models

### Recommended Models (Cost-Effective)

| Model | Provider | Input Price | Output Price | Context | Use Case |
|-------|----------|-------------|--------------|---------|----------|
| `openai/gpt-4o-mini` | OpenAI | $0.15/1M | $0.60/1M | 128K | **Default** - Balanced |
| `google/gemini-flash-1.5` | Google | $0.075/1M | $0.30/1M | 1M | High volume, cheap |
| `anthropic/claude-3-haiku` | Anthropic | $0.25/1M | $1.25/1M | 200K | Fast responses |
| `meta-llama/llama-3.1-8b-instruct` | Meta | $0.06/1M | $0.06/1M | 128K | Cheapest option |

### Premium Models (High Quality)

| Model | Provider | Input Price | Output Price | Context | Use Case |
|-------|----------|-------------|--------------|---------|----------|
| `openai/gpt-4o` | OpenAI | $2.50/1M | $10.00/1M | 128K | Complex reasoning |
| `anthropic/claude-3.5-sonnet` | Anthropic | $3.00/1M | $15.00/1M | 200K | Best quality |
| `google/gemini-pro-1.5` | Google | $1.25/1M | $5.00/1M | 2M | Long context |

**Lihat semua model**: https://openrouter.ai/models

## Model Selection Strategy

### 1. By Use Case

**Customer Service (Default)**
```bash
OPENROUTER_MODEL=openai/gpt-4o-mini
OPENROUTER_TEMPERATURE=0.3
```
- Balanced cost dan quality
- Good reasoning
- Fast response

**High Volume / Cost Sensitive**
```bash
OPENROUTER_MODEL=google/gemini-flash-1.5
OPENROUTER_TEMPERATURE=0.2
```
- Cheapest option
- Good for simple FAQs
- Very fast

**Premium / Complex Queries**
```bash
OPENROUTER_MODEL=anthropic/claude-3.5-sonnet
OPENROUTER_TEMPERATURE=0.4
```
- Best quality
- Complex reasoning
- Nuanced responses

### 2. By Language

**Bahasa Indonesia (Optimized)**
```bash
OPENROUTER_MODEL=openai/gpt-4o-mini
# or
OPENROUTER_MODEL=google/gemini-flash-1.5
```

**Multi-language Support**
```bash
OPENROUTER_MODEL=anthropic/claude-3-haiku
```

## Request Parameters

### Temperature (Kreativitas)

Controls randomness in responses:

```bash
# Sangat konsisten (FAQ, technical support)
OPENROUTER_TEMPERATURE=0.1

# Balanced (default, recommended)
OPENROUTER_TEMPERATURE=0.3

# Kreatif (marketing, copywriting)
OPENROUTER_TEMPERATURE=0.7

# Sangat kreatif (brainstorming)
OPENROUTER_TEMPERATURE=1.0
```

### Max Tokens (Panjang Response)

Limit output length:

```bash
# Short answers (50-100 tokens)
OPENROUTER_MAX_TOKENS=150

# Medium answers (default, 200-400 tokens)
OPENROUTER_MAX_TOKENS=500

# Long answers (500+ tokens)
OPENROUTER_MAX_TOKENS=1500
```

**Note**: 1 token ≈ 4 characters atau ≈ 0.75 kata

### Top P (Diversity)

Controls diversity via nucleus sampling:

```bash
# Fokus pada token paling probable (konsisten)
OPENROUTER_TOP_P=0.5

# Balanced (default)
OPENROUTER_TOP_P=1.0
```

### Frequency/Presence Penalty (Repetition)

Mengurangi pengulangan:

```bash
# Kurangi repetisi kata yang sering muncul
OPENROUTER_FREQUENCY_PENALTY=0.5

# Kurangi repetisi kata yang sudah muncul
OPENROUTER_PRESENCE_PENALTY=0.5
```

## Advanced Features

### 1. Fallback Routing

OpenRouter otomatis fallback ke provider lain jika:
- Rate limited
- Provider down (5xx error)
- Timeout

**No configuration needed** - berjalan otomatis!

### 2. Cost Tracking

Track token usage via response:

```go
resp, err := client.CreateChatCompletion(ctx, req)

// Token counts available in response
inputTokens := resp.Usage.PromptTokens
outputTokens := resp.Usage.CompletionTokens
totalTokens := resp.Usage.TotalTokens
```

### 3. Generation Stats

Query detailed stats setelah request:

```bash
curl https://openrouter.ai/api/v1/generation?id=$GENERATION_ID \
  -H "Authorization: Bearer $OPENROUTER_API_KEY"
```

Response:
```json
{
  "id": "gen-xxxxx",
  "model": "openai/gpt-4o-mini",
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 50,
    "total_tokens": 150
  },
  "native_tokens_prompt": 100,
  "native_tokens_completion": 50,
  "total_cost": 0.000075
}
```

## Implementation Details

### Current Implementation (openrouter.go)

```go
// ✅ Implemented
- Base URL: https://openrouter.ai/api/v1
- Custom headers: HTTP-Referer, X-Title
- Temperature: configurable (default 0.3)
- Model: configurable (default gpt-4o-mini)
- Token counting: dari response usage

// ⏭️ Future enhancements
- Max tokens parameter
- Top P parameter
- Frequency/Presence penalty
- Streaming support
- Tool calling
```

### Headers Sent

```http
POST /api/v1/chat/completions HTTP/1.1
Host: openrouter.ai
Authorization: Bearer sk-or-v1-xxxxx
HTTP-Referer: https://clivy.app
X-Title: Clivy
Content-Type: application/json
```

### Request Body

```json
{
  "model": "openai/gpt-4o-mini",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful customer service..."
    },
    {
      "role": "user",
      "content": "Hi, I need help with..."
    }
  ],
  "temperature": 0.3
}
```

### Response Format

```json
{
  "id": "gen-xxxxxx",
  "model": "openai/gpt-4o-mini",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Hello! I'd be happy to help..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 120,
    "completion_tokens": 45,
    "total_tokens": 165
  }
}
```

## Cost Estimation

### Example Scenario

**Configuration:**
- Model: `openai/gpt-4o-mini`
- Average prompt: 500 tokens (system + history + user)
- Average response: 200 tokens
- Volume: 1000 conversations/day

**Monthly Cost:**

```
Input:  500 tokens × 1000 × 30 days = 15M tokens
        15M × $0.15/1M = $2.25

Output: 200 tokens × 1000 × 30 days = 6M tokens
        6M × $0.60/1M = $3.60

Total:  $5.85/month untuk 30,000 conversations
        ≈ $0.000195 per conversation
```

### Cost Optimization Tips

1. **Gunakan model yang tepat:**
   - FAQ sederhana → `gemini-flash-1.5` ($0.075/1M input)
   - General support → `gpt-4o-mini` ($0.15/1M input)
   - Complex issues → `claude-3-haiku` ($0.25/1M input)

2. **Optimize prompt length:**
   - Kirim hanya 10 pesan terakhir (bukan full history)
   - Compress knowledge base context
   - Use concise system prompts

3. **Set max_tokens:**
   - Short answers: 150 tokens
   - Medium: 500 tokens
   - Only set higher if needed

4. **Cache frequent responses:**
   - FAQ cache di database
   - Fallback text untuk common errors

## Testing

### 1. Test API Key

```bash
curl https://openrouter.ai/api/v1/auth/key \
  -H "Authorization: Bearer $OPENROUTER_API_KEY"
```

Expected:
```json
{
  "data": {
    "label": "Your Key Name",
    "usage": 0.05,
    "limit": null,
    "is_free_tier": false
  }
}
```

### 2. Test Model Availability

```bash
curl https://openrouter.ai/api/v1/models \
  -H "Authorization: Bearer $OPENROUTER_API_KEY"
```

### 3. Test Chat Completion

```bash
curl https://openrouter.ai/api/v1/chat/completions \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "HTTP-Referer: https://clivy.app" \
  -H "X-Title: Clivy" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "Say hello in Indonesian"}
    ]
  }'
```

## Monitoring & Debugging

### 1. Check Credit Balance

Dashboard: https://openrouter.ai/credits

### 2. View Usage Stats

Dashboard: https://openrouter.ai/activity

### 3. Debug Logs

Enable detailed logging:

```go
// In openrouter.go
import "log"

func AskLLM(ctx context.Context, ...) {
    log.Printf("[OpenRouter] Model: %s", model)
    log.Printf("[OpenRouter] Temperature: %.2f", temperature)
    log.Printf("[OpenRouter] Request: %+v", req)
    
    resp, err := client.CreateChatCompletion(ctx, req)
    
    log.Printf("[OpenRouter] Response tokens: %d in, %d out", 
        resp.Usage.PromptTokens, 
        resp.Usage.CompletionTokens)
}
```

### 4. Error Handling

Common errors:

| Error | Cause | Solution |
|-------|-------|----------|
| 401 Unauthorized | Invalid API key | Check OPENROUTER_API_KEY |
| 402 Payment Required | Insufficient credits | Add credits |
| 429 Rate Limited | Too many requests | Enable fallback routing |
| 500 Server Error | Provider issue | Automatic fallback |
| Context too long | Prompt > model limit | Reduce history/knowledge |

## Best Practices

### 1. System Prompt Optimization

**❌ Too verbose:**
```
You are a highly intelligent and sophisticated AI assistant 
designed to provide exceptional customer service with empathy 
and professionalism...
```

**✅ Concise:**
```
You are Clivy CS assistant. Answer briefly in Indonesian. 
Use knowledge base. Be helpful and professional.
```

### 2. Context Management

**❌ Send all history:**
```go
// 100 messages × 50 tokens = 5000 tokens wasted
messages := getAllChatHistory(sessionID)
```

**✅ Recent context only:**
```go
// Last 10 messages × 50 tokens = 500 tokens
messages := getRecentMessages(sessionID, 10)
```

### 3. Temperature Settings

| Use Case | Temperature | Reason |
|----------|-------------|--------|
| FAQ lookups | 0.1 - 0.2 | Consistent answers |
| Customer support | 0.3 - 0.4 | Balanced |
| Product recommendations | 0.5 - 0.7 | Creative suggestions |
| Content generation | 0.8 - 1.0 | Diverse outputs |

### 4. Error Handling

```go
resp, err := AskLLM(ctx, client, sysPrompt, userMsg)
if err != nil {
    // Log error with context
    log.Printf("[AI Error] Model: %s, UserID: %s, Error: %v", 
        model, userID, err)
    
    // Use fallback
    return bot.FallbackText, nil
}
```

## Troubleshooting

### Issue: High costs

**Solutions:**
1. Switch to cheaper model (`gemini-flash-1.5`)
2. Reduce max_tokens
3. Optimize prompt length
4. Cache common responses

### Issue: Slow responses

**Solutions:**
1. Use faster model (`gpt-4o-mini` atau `gemini-flash`)
2. Reduce AI_TIMEOUT_MS
3. Use streaming (future)
4. Deploy worker closer to OpenRouter

### Issue: Poor quality responses

**Solutions:**
1. Improve system prompt
2. Add more knowledge base context
3. Increase temperature (0.3 → 0.5)
4. Use better model (`claude-3-haiku`)

### Issue: Context length errors

**Solutions:**
1. Reduce message history (10 → 5)
2. Compress knowledge base
3. Use model dengan context lebih besar
4. Truncate old messages

## Resources

- **OpenRouter Docs**: https://openrouter.ai/docs
- **Model Pricing**: https://openrouter.ai/models
- **API Reference**: https://openrouter.ai/docs/api-reference
- **Dashboard**: https://openrouter.ai/
- **Discord Support**: https://discord.gg/openrouter

## Changelog

### v1.0 (Current)
- ✅ Basic chat completion
- ✅ Custom headers (Referer, Title)
- ✅ Configurable model & temperature
- ✅ Token usage tracking

### v1.1 (Planned)
- ⏭️ Streaming support
- ⏭️ Additional parameters (top_p, max_tokens, penalties)
- ⏭️ Tool calling integration
- ⏭️ Multiple model fallback
- ⏭️ Response caching
