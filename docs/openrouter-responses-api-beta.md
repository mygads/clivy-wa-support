# OpenRouter Responses API Beta

## ⚠️ Status: Beta - Not Recommended for Production

**Last Updated**: November 12, 2025  
**Current Implementation**: Chat Completions API (Stable)  
**Beta API**: Responses API (Not Implemented)

## Overview

OpenRouter menyediakan **dua API berbeda**:

| Feature | Chat Completions API | Responses API Beta |
|---------|---------------------|-------------------|
| **URL** | `/v1/chat/completions` | `/v1/responses` |
| **Status** | ✅ Stable, Production-Ready | ⚠️ Beta, May Change |
| **Format** | OpenAI-compatible messages | Structured input/output |
| **Features** | Standard chat | Reasoning, Tools, Web Search |
| **Stateful** | Stateless | Stateless |
| **SDK Support** | ✅ go-openai SDK | ❌ Custom implementation |
| **Clivy Status** | ✅ **IMPLEMENTED** | ❌ Not implemented |

## Current Implementation (RECOMMENDED)

Clivy menggunakan **Chat Completions API** yang sudah stabil dan battle-tested:

```go
// services/openrouter.go (CURRENT)
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
```

**Why This Works:**
✅ Compatible dengan go-openai SDK  
✅ Stabil tanpa breaking changes  
✅ Cukup untuk customer service bot  
✅ Token tracking akurat  
✅ Sudah terintegrasi dengan worker  

## Responses API Beta Format

### Request Format (Different from Current)

```json
POST /api/v1/responses
{
  "model": "openai/o4-mini",
  "input": "What is OpenRouter?",
  "max_output_tokens": 9000,
  "stream": false,
  "reasoning": {
    "effort": "high"
  },
  "tools": [],
  "plugins": [{"id": "web", "max_results": 3}]
}
```

### Response Format (Different from Current)

```json
{
  "id": "resp_1234567890",
  "object": "response",
  "created_at": 1234567890,
  "model": "openai/o4-mini",
  "output": [
    {
      "type": "message",
      "id": "msg_abc123",
      "status": "completed",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "OpenRouter is a unified API...",
          "annotations": []
        }
      ]
    }
  ],
  "usage": {
    "input_tokens": 12,
    "output_tokens": 45,
    "total_tokens": 57
  },
  "status": "completed"
}
```

## Feature Comparison

### 1. Reasoning Capabilities

**Responses API Beta:**
```json
{
  "model": "openai/o4-mini",
  "input": "Was 1995 30 years ago?",
  "reasoning": {
    "effort": "high"
  }
}
```

**Response includes reasoning chain:**
```json
{
  "output": [
    {
      "type": "reasoning",
      "id": "rs_abc123",
      "encrypted_content": "gAAA...",
      "summary": [
        "First, I need to determine the current year",
        "Then calculate the difference from 1995"
      ]
    },
    {
      "type": "message",
      "content": [{"type": "output_text", "text": "Yes, in 2025..."}]
    }
  ]
}
```

**Chat Completions API (Current):**
```json
{
  "messages": [
    {"role": "system", "content": "Think step by step."},
    {"role": "user", "content": "Was 1995 30 years ago?"}
  ]
}
```

**Response:**
```json
{
  "choices": [{
    "message": {
      "content": "Let me think... 2025 - 1995 = 30 years. Yes!"
    }
  }]
}
```

**Verdict for Customer Service:** Current implementation sudah cukup! Reasoning chains tidak kritis untuk CS bot.

### 2. Tool Calling

**Responses API Beta:**
```json
{
  "tools": [{
    "type": "function",
    "name": "get_weather",
    "parameters": {
      "type": "object",
      "properties": {
        "location": {"type": "string"}
      }
    }
  }]
}
```

**Chat Completions API (Current):**
```json
{
  "functions": [{
    "name": "get_weather",
    "parameters": {
      "type": "object",
      "properties": {
        "location": {"type": "string"}
      }
    }
  }],
  "function_call": "auto"
}
```

**Verdict:** Both APIs support function calling! Format sedikit berbeda tapi fitur sama.

### 3. Web Search

**Responses API Beta:**
```json
{
  "input": "What is the latest news?",
  "plugins": [{"id": "web", "max_results": 3}]
}
```

**Chat Completions API (Current):**
```json
{
  "model": "openai/o4-mini:online",
  "messages": [{"role": "user", "content": "What is the latest news?"}]
}
```

**Verdict:** Both support web search! Chat Completions API menggunakan `:online` suffix.

### 4. Streaming

**Responses API Beta:**
```json
{
  "stream": true
}
```

Streaming format berbeda (Server-Sent Events):
```
data: {"type":"response.created"}
data: {"type":"response.content_part.delta","delta":"Hello"}
data: [DONE]
```

**Chat Completions API (Current):**
```json
{
  "stream": true
}
```

Streaming format standar:
```
data: {"choices":[{"delta":{"content":"Hello"}}]}
data: [DONE]
```

**Verdict:** Both support streaming. Format berbeda tapi SDK handle otomatis.

## Migration Path (If Needed in Future)

### Step 1: Create New Service

```go
// services/openrouter_responses.go (NEW FILE - NOT CREATED YET)
package services

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
)

type ResponsesAPIRequest struct {
    Model           string                 `json:"model"`
    Input           interface{}            `json:"input"` // string or []Message
    MaxOutputTokens int                    `json:"max_output_tokens,omitempty"`
    Temperature     float64                `json:"temperature,omitempty"`
    Stream          bool                   `json:"stream,omitempty"`
    Reasoning       *ReasoningConfig       `json:"reasoning,omitempty"`
    Tools           []Tool                 `json:"tools,omitempty"`
    Plugins         []Plugin               `json:"plugins,omitempty"`
}

type ReasoningConfig struct {
    Effort string `json:"effort"` // minimal, low, medium, high
}

type Tool struct {
    Type       string                 `json:"type"` // "function"
    Name       string                 `json:"name"`
    Parameters map[string]interface{} `json:"parameters"`
}

type Plugin struct {
    ID         string `json:"id"` // "web"
    MaxResults int    `json:"max_results,omitempty"`
}

type ResponsesAPIResponse struct {
    ID        string   `json:"id"`
    Object    string   `json:"object"`
    CreatedAt int64    `json:"created_at"`
    Model     string   `json:"model"`
    Output    []Output `json:"output"`
    Usage     Usage    `json:"usage"`
    Status    string   `json:"status"`
}

type Output struct {
    Type    string    `json:"type"` // "message", "reasoning", "function_call"
    ID      string    `json:"id"`
    Status  string    `json:"status"`
    Role    string    `json:"role"`
    Content []Content `json:"content"`
}

type Content struct {
    Type        string       `json:"type"` // "output_text"
    Text        string       `json:"text"`
    Annotations []Annotation `json:"annotations"`
}

type Annotation struct {
    Type       string `json:"type"` // "url_citation"
    URL        string `json:"url"`
    StartIndex int    `json:"start_index"`
    EndIndex   int    `json:"end_index"`
}

type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
    TotalTokens  int `json:"total_tokens"`
}

func AskLLMResponsesAPI(ctx context.Context, systemPrompt, userMessage string) (string, int, int, error) {
    apiKey := os.Getenv("OPENROUTER_API_KEY")
    if apiKey == "" {
        return "", 0, 0, fmt.Errorf("OPENROUTER_API_KEY not set")
    }

    model := os.Getenv("OPENROUTER_MODEL")
    if model == "" {
        model = "openai/o4-mini"
    }

    // Build request (simple string input)
    reqData := ResponsesAPIRequest{
        Model:           model,
        Input:           systemPrompt + "\n\n" + userMessage, // Combine prompts
        MaxOutputTokens: 9000,
        Temperature:     0.3,
    }

    jsonData, err := json.Marshal(reqData)
    if err != nil {
        return "", 0, 0, err
    }

    req, err := http.NewRequestWithContext(ctx, "POST", 
        "https://openrouter.ai/api/v1/responses", 
        bytes.NewBuffer(jsonData))
    if err != nil {
        return "", 0, 0, err
    }

    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("HTTP-Referer", os.Getenv("OPENROUTER_HTTP_REFERER"))
    req.Header.Set("X-Title", os.Getenv("OPENROUTER_X_TITLE"))

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return "", 0, 0, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return "", 0, 0, fmt.Errorf("API error %d: %s", resp.StatusCode, body)
    }

    var apiResp ResponsesAPIResponse
    if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
        return "", 0, 0, err
    }

    // Extract text from output
    var output string
    for _, out := range apiResp.Output {
        if out.Type == "message" && len(out.Content) > 0 {
            for _, content := range out.Content {
                if content.Type == "output_text" {
                    output = content.Text
                    break
                }
            }
        }
    }

    if output == "" {
        return "", 0, 0, fmt.Errorf("no text output in response")
    }

    return output, apiResp.Usage.InputTokens, apiResp.Usage.OutputTokens, nil
}
```

### Step 2: Update Worker

```go
// worker/ai_worker.go (MODIFICATION)

// Option 1: Use existing Chat Completions API (CURRENT)
response, inTok, outTok, err := services.AskLLM(timeoutCtx, w.llmClient, ctx.SystemPrompt, ctx.UserMessage)

// Option 2: Use Responses API Beta (FUTURE)
response, inTok, outTok, err := services.AskLLMResponsesAPI(timeoutCtx, ctx.SystemPrompt, ctx.UserMessage)
```

### Step 3: Environment Variable

```bash
# .env
USE_RESPONSES_API_BETA=false  # Default: false (use stable API)
```

## Should We Migrate?

### ❌ Reasons NOT to Migrate

1. **Beta Status**: API masih beta, bisa ada breaking changes
2. **No SDK Support**: go-openai SDK tidak support Responses API
3. **Custom Implementation**: Harus maintain custom HTTP client
4. **No Additional Value**: Fitur yang kita butuhkan sudah ada di Chat Completions API
5. **Added Complexity**: Parsing response lebih kompleks
6. **Risk**: Production stability lebih penting

### ✅ Reasons to Migrate (In Future)

1. **Advanced Reasoning**: Jika butuh encrypted reasoning chains
2. **Structured Annotations**: Jika butuh citation tracking dengan start/end index
3. **Plugin System**: Jika butuh multiple plugins (web search + custom tools)
4. **Future Features**: Responses API akan dapat fitur baru lebih dulu

## Recommendation

### For Clivy AI Bot (Current Use Case)

**STICK WITH CHAT COMPLETIONS API** ✅

**Reasons:**
- Customer service bot tidak butuh advanced reasoning
- Chat Completions API sudah production-ready
- go-openai SDK fully supported
- Simpler error handling
- Lower maintenance overhead
- All needed features available (function calling, streaming)

### When to Consider Migration

**ONLY IF** Anda butuh:
- Encrypted reasoning chains untuk transparency
- Structured web search citations
- Multiple plugin integration
- Future beta features yang tidak ada di Chat Completions API

**AND** Anda siap untuk:
- Maintain custom HTTP client
- Handle beta API breaking changes
- Write extensive tests
- Monitor for API updates

## Testing Responses API Beta (Optional)

Jika ingin test secara manual:

```bash
# Test basic request
curl -X POST https://openrouter.ai/api/v1/responses \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/o4-mini",
    "input": "Hello, how are you?",
    "max_output_tokens": 9000
  }'
```

```bash
# Test with reasoning
curl -X POST https://openrouter.ai/api/v1/responses \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/o4-mini",
    "input": "Was 1995 30 years ago?",
    "reasoning": {"effort": "high"},
    "max_output_tokens": 9000
  }'
```

```bash
# Test with web search
curl -X POST https://openrouter.ai/api/v1/responses \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/o4-mini",
    "input": "What is OpenRouter?",
    "plugins": [{"id": "web", "max_results": 3}],
    "max_output_tokens": 9000
  }'
```

## Summary

| Aspect | Current (Chat Completions) | Responses API Beta |
|--------|---------------------------|-------------------|
| **Status** | ✅ Production | ⚠️ Beta |
| **Implementation** | ✅ Complete | ❌ Not implemented |
| **SDK Support** | ✅ go-openai | ❌ Custom |
| **Features Needed** | ✅ All available | ❓ Extra features unused |
| **Stability** | ✅ Stable | ⚠️ May change |
| **Complexity** | ✅ Simple | ❌ Complex |
| **Maintenance** | ✅ Low | ❌ High |
| **Recommendation** | ✅ **USE THIS** | ❌ Wait for stable release |

## Conclusion

**DO NOT MIGRATE** untuk sekarang. Chat Completions API sudah sempurna untuk use case Clivy AI Bot.

**MONITOR** Responses API Beta development. Jika API sudah stable (v1.0), bisa dipertimbangkan untuk fitur advanced.

**FOCUS** pada optimization yang sudah ada:
- ✅ Error handling improvement (circuit breaker, retry logic)
- ✅ Cost optimization (prompt compression, model selection)
- ✅ Performance monitoring (latency, token usage)
- ✅ Feature development (UI, knowledge base, analytics)

---

**Related Documentation:**
- [OpenRouter Configuration Guide](./openrouter-config.md)
- [OpenRouter Quick Start](./openrouter-quickstart.md)
- [OpenRouter Error Handling](./openrouter-errors.md)

**Official Resources:**
- Responses API Beta Docs: https://openrouter.ai/docs/api-reference/responses
- Chat Completions API Docs: https://openrouter.ai/docs/api-reference/chat
