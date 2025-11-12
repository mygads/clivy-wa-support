# OpenRouter Quick Start Guide

## 1. Get API Key

1. Buka https://openrouter.ai/
2. Sign in / Register
3. Go to **Keys** → **Create Key**
4. Copy API key (starts with `sk-or-v1-`)

## 2. Add Credits

1. Go to **Credits**
2. Add $5 minimum (cukup untuk ~100,000 requests dengan gpt-4o-mini)
3. Credits tidak expired

## 3. Configure Environment

Edit `.env` di `clivy-wa-support`:

```bash
# Required
OPENROUTER_API_KEY=sk-or-v1-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
OPENROUTER_MODEL=openai/gpt-4o-mini
AI_TIMEOUT_MS=120000

# Optional (recommended)
OPENROUTER_HTTP_REFERER=https://clivy.app
OPENROUTER_X_TITLE=Clivy
```

## 4. Test Configuration

### Test API Key
```bash
curl https://openrouter.ai/api/v1/auth/key \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### Test Chat
```bash
curl https://openrouter.ai/api/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hi"}]
  }'
```

## 5. Recommended Models

### For Customer Service (Default)
```bash
OPENROUTER_MODEL=openai/gpt-4o-mini
```
- **Cost**: $0.15/1M input, $0.60/1M output
- **Quality**: Good
- **Speed**: Fast
- **Use**: General customer support

### For High Volume
```bash
OPENROUTER_MODEL=google/gemini-flash-1.5
```
- **Cost**: $0.075/1M input, $0.30/1M output
- **Quality**: Good
- **Speed**: Very fast
- **Use**: Simple FAQs, high traffic

### For Premium Quality
```bash
OPENROUTER_MODEL=anthropic/claude-3-haiku
```
- **Cost**: $0.25/1M input, $1.25/1M output
- **Quality**: Excellent
- **Speed**: Fast
- **Use**: Complex queries, premium service

## 6. Optimize Costs

### Reduce Prompt Size
```go
// ❌ Bad: Send all 100 messages
messages := GetAllMessages(sessionID)

// ✅ Good: Last 10 only
messages := GetRecentMessages(sessionID, 10)
```

### Set Max Tokens
```bash
# Short answers (save $$$)
OPENROUTER_MAX_TOKENS=200

# Medium answers
OPENROUTER_MAX_TOKENS=500
```

### Use Cheaper Models for Simple Tasks
```bash
# FAQ: Use cheapest
if isFAQ(message) {
    model = "google/gemini-flash-1.5"
}

# Complex: Use better model
if isComplexQuery(message) {
    model = "openai/gpt-4o-mini"
}
```

## 7. Monitor Usage

### Dashboard
https://openrouter.ai/activity

Shows:
- Requests per day
- Token usage
- Cost breakdown
- Model distribution

### Check Balance
```bash
curl https://openrouter.ai/api/v1/auth/key \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## 8. Common Issues

### "401 Unauthorized"
- ❌ Invalid API key
- ✅ Check `OPENROUTER_API_KEY` in `.env`

### "402 Payment Required"
- ❌ Insufficient credits
- ✅ Add credits at https://openrouter.ai/credits

### "Context length exceeded"
- ❌ Prompt too long
- ✅ Reduce message history (10 → 5)
- ✅ Shorten knowledge base context

### High costs
- ❌ Using expensive model
- ✅ Switch to `gemini-flash-1.5`
- ✅ Reduce max_tokens
- ✅ Send less history

## 9. Cost Examples

### Scenario: 1000 conversations/day

**With gpt-4o-mini:**
```
Input:  500 tokens × 1000 × 30 = 15M
        15M × $0.15/1M = $2.25

Output: 200 tokens × 1000 × 30 = 6M  
        6M × $0.60/1M = $3.60

Total: $5.85/month
```

**With gemini-flash-1.5:**
```
Input:  500 tokens × 1000 × 30 = 15M
        15M × $0.075/1M = $1.13

Output: 200 tokens × 1000 × 30 = 6M
        6M × $0.30/1M = $1.80

Total: $2.93/month (50% cheaper!)
```

## 10. Next Steps

- ✅ Configure API key
- ✅ Test with sample request
- ✅ Create bot in UI
- ✅ Upload knowledge base
- ✅ Bind to WhatsApp session
- ✅ Monitor costs in dashboard

## Resources

- **Full Documentation**: [openrouter-config.md](./openrouter-config.md)
- **Model Prices**: https://openrouter.ai/models
- **API Docs**: https://openrouter.ai/docs
- **Support**: https://discord.gg/openrouter

---

**Ready to start?** Just set `OPENROUTER_API_KEY` in your `.env` and restart the service! 🚀
