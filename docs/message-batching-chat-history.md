# Message Batching & Chat History Implementation Plan

## Problem 1: Bubble Messages (Multiple Sequential Messages)

### Current Behavior
```
User sends:
- Message 1: "Halo"          → Job #1 created
- Message 2: "Saya mau"      → Job #2 created  
- Message 3: "pesan produk"  → Job #3 created

Result: 3 LLM calls, high token usage, potentially confusing responses
```

### Proposed Solution: Debouncing with 3-Second Window

```
User sends:
- Message 1: "Halo"          → Start timer (3s)
- Message 2: "Saya mau"      → Reset timer (3s)
- Message 3: "pesan produk"  → Reset timer (3s)
- [3 seconds pass, no new message]
- → Job #1 created with all 3 messages batched

Result: 1 LLM call with combined context, coherent response
```

### Implementation Strategy

#### Option A: Database-Based Debouncing (Recommended)
**Pros:** No Redis, simpler, works with LISTEN/NOTIFY
**Cons:** Slight delay (polling-based)

```sql
-- Add fields to ai_chat_messages:
ALTER TABLE ai_chat_messages ADD COLUMN is_batched BOOLEAN DEFAULT FALSE;
ALTER TABLE ai_chat_messages ADD COLUMN batch_id INTEGER;
ALTER TABLE ai_chat_messages ADD COLUMN last_activity TIMESTAMP;

-- Add fields to ai_jobs:
ALTER TABLE ai_jobs ADD COLUMN is_batch BOOLEAN DEFAULT FALSE;
ALTER TABLE ai_jobs ADD COLUMN batched_msg_ids TEXT;
ALTER TABLE ai_jobs ADD COLUMN batch_size INTEGER DEFAULT 1;

-- Index for performance
CREATE INDEX idx_ai_chat_messages_batching ON ai_chat_messages(session_tok, "from", is_batched, last_activity);
```

**Workflow:**
1. Message arrives → Save to `ai_chat_messages` with `last_activity = NOW()`
2. **Do NOT create job immediately**
3. Background worker (separate goroutine) runs every 1 second:
   ```sql
   SELECT session_tok, "from", COUNT(*), array_agg(message_id)
   FROM ai_chat_messages
   WHERE is_batched = FALSE 
     AND last_activity < NOW() - INTERVAL '3 seconds'
   GROUP BY session_tok, "from"
   ```
4. For each group → Create 1 batch job with all message IDs
5. Mark messages as `is_batched = TRUE`, `batch_id = job.ID`

#### Option B: In-Memory Debouncing (Alternative)
**Pros:** Real-time, precise timing
**Cons:** Lost on restart, harder to debug

```go
// In ai_webhook.go
type MessageBatch struct {
    Messages  []models.AIChatMessage
    Timer     *time.Timer
    mu        sync.Mutex
}

var (
    batches = make(map[string]*MessageBatch) // key: sessionTok_from
    batchMu sync.RWMutex
)

func HandleAIWebhook(c *gin.Context) {
    // ... existing code to save message ...
    
    // Add to batch instead of creating job immediately
    batchKey := fmt.Sprintf("%s_%s", sessionToken, from)
    
    batchMu.Lock()
    batch, exists := batches[batchKey]
    if !exists {
        batch = &MessageBatch{
            Messages: []models.AIChatMessage{},
        }
        batches[batchKey] = batch
    }
    
    batch.mu.Lock()
    batch.Messages = append(batch.Messages, chatMsg)
    
    // Reset or create timer
    if batch.Timer != nil {
        batch.Timer.Stop()
    }
    
    batch.Timer = time.AfterFunc(3*time.Second, func() {
        processBatch(batchKey)
    })
    
    batch.mu.Unlock()
    batchMu.Unlock()
}

func processBatch(batchKey string) {
    batchMu.Lock()
    batch := batches[batchKey]
    delete(batches, batchKey)
    batchMu.Unlock()
    
    // Create single job for all messages
    // ... create AIJob with batched_msg_ids ...
}
```

### Recommendation: **Option A (Database-Based)**
- Simpler to implement
- Survives restarts
- Easier to debug
- Works well with existing LISTEN/NOTIFY

---

## Problem 2: Chat History not Saved to ChatRoom

### Current Behavior
- Messages only saved to `ai_chat_messages` (temporary, for AI context)
- NOT saved to `chat_rooms` & `chat_messages` (permanent history)
- Users can't see chat history in UI

### Proposed Solution: Dual Save

```go
// In ai_webhook.go - after saving to ai_chat_messages

// 1. Save to ai_chat_messages (existing)
db.Create(&chatMsg) // ✅ Already done

// 2. Save to chat_rooms & chat_messages (NEW)
saveToChatHistory(sessionToken, from, to, body, pushName, timestamp, fromMe)
```

### Implementation

```go
// In handlers/ai_webhook.go or services/chat_history.go

func saveToChatHistory(sessionToken, from, to, body, pushName string, timestamp time.Time, fromMe bool) error {
    db := database.GetDB()
    
    // 1. Find or create ChatRoom
    chatID := fmt.Sprintf("%s_%s", sessionToken, from)
    
    var chatRoom models.ChatRoom
    err := db.Where("chat_id = ?", chatID).First(&chatRoom).Error
    if err != nil {
        // Create new chat room
        chatRoom = models.ChatRoom{
            ChatID:       chatID,
            UserToken:    sessionToken,
            ContactJID:   from,
            ContactName:  pushName,
            ChatType:     "individual",
            IsGroup:      false,
            LastMessage:  body,
            LastSender:   getSenderType(fromMe),
            LastActivity: timestamp,
            UnreadCount:  getUnreadIncrement(fromMe),
            CreatedAt:    time.Now(),
            UpdatedAt:    time.Now(),
        }
        if err := db.Create(&chatRoom).Error; err != nil {
            return fmt.Errorf("failed to create chat room: %w", err)
        }
    } else {
        // Update existing chat room
        updates := map[string]interface{}{
            "last_message":  body,
            "last_sender":   getSenderType(fromMe),
            "last_activity": timestamp,
        }
        if !fromMe {
            updates["unread_count"] = gorm.Expr("unread_count + ?", 1)
        }
        db.Model(&chatRoom).Updates(updates)
    }
    
    // 2. Save to ChatMessage
    chatMessage := models.ChatMessage{
        MessageID:        generateMessageID(), // or use WA message ID
        ChatRoomID:       chatRoom.ID,
        ChatID:           chatID,
        UserToken:        sessionToken,
        SenderJID:        from,
        SenderType:       getSenderType(fromMe),
        MessageType:      "text",
        Content:          body,
        Status:           "sent",
        MessageTimestamp: timestamp,
        CreatedAt:        time.Now(),
        UpdatedAt:        time.Now(),
    }
    
    if err := db.Create(&chatMessage).Error; err != nil {
        return fmt.Errorf("failed to save chat message: %w", err)
    }
    
    return nil
}

func getSenderType(fromMe bool) string {
    if fromMe {
        return "user"
    }
    return "contact"
}

func getUnreadIncrement(fromMe bool) int {
    if fromMe {
        return 0 // Pesan dari user tidak menambah unread
    }
    return 1 // Pesan dari contact +1 unread
}
```

### When to Call?

**Option 1: Save immediately on webhook** (Recommended)
```go
// In HandleAIWebhook, after saving to ai_chat_messages
if err := db.Create(&chatMsg).Error; err == nil {
    // Save to chat history immediately
    go saveToChatHistory(sessionToken, from, to, body, pushName, timestamp, fromMe)
}
```

**Option 2: Save after AI response**
```go
// In worker, after sending AI reply
if err := services.SendWAText(...); err == nil {
    // Save AI response to chat history
    go saveToChatHistory(sessionToken, to, from, response, "AI Bot", time.Now(), true)
}
```

**Best: Do BOTH**
- Save incoming message immediately (Option 1)
- Save AI response after sending (Option 2)

---

## Implementation Priority

### Phase 1: Chat History (High Priority) ✅
1. Create `saveToChatHistory()` function
2. Call it in `HandleAIWebhook()` for incoming messages
3. Call it in `ai_worker.go` after sending AI reply
4. Test: Verify messages appear in UI chat history

### Phase 2: Message Batching (Medium Priority) ⏭️
1. Add migration for new fields in `ai_chat_messages` and `ai_jobs`
2. Create background debouncer goroutine
3. Modify `HandleAIWebhook()` to skip job creation
4. Test: Send 3 messages quickly, verify 1 job created with all 3

### Phase 3: UI Integration (Low Priority) ⏭️
1. Create chat history page in Next.js
2. API endpoint to fetch ChatRoom list
3. API endpoint to fetch ChatMessage by room
4. Real-time updates (optional)

---

## Migration SQL

```sql
-- Phase 2: Batching support
ALTER TABLE ai_chat_messages 
ADD COLUMN is_batched BOOLEAN DEFAULT FALSE,
ADD COLUMN batch_id INTEGER,
ADD COLUMN last_activity TIMESTAMP DEFAULT NOW();

CREATE INDEX idx_ai_chat_batching 
ON ai_chat_messages(session_tok, "from", is_batched, last_activity);

ALTER TABLE ai_jobs
ADD COLUMN is_batch BOOLEAN DEFAULT FALSE,
ADD COLUMN batched_msg_ids TEXT,
ADD COLUMN batch_size INTEGER DEFAULT 1;
```

---

## Testing Plan

### Test 1: Chat History
```bash
# 1. Send message via webhook
curl -X POST http://localhost:8070/webhook/ai \
  -H "Content-Type: application/json" \
  -d @test_payload.json

# 2. Check chat_rooms table
psql -d clivy_support -c "SELECT * FROM chat_rooms ORDER BY last_activity DESC LIMIT 5;"

# 3. Check chat_messages table
psql -d clivy_support -c "SELECT * FROM chat_messages ORDER BY message_timestamp DESC LIMIT 10;"

# 4. Verify AI response also saved
# (after worker processes job)
```

### Test 2: Message Batching
```bash
# 1. Send 3 messages quickly (< 1 second apart)
# Message 1, 2, 3

# 2. Wait 4 seconds

# 3. Check ai_chat_messages
psql -d clivy_support -c "SELECT id, body, is_batched, batch_id FROM ai_chat_messages WHERE is_batched = TRUE;"

# 4. Check ai_jobs
psql -d clivy_support -c "SELECT id, is_batch, batch_size, batched_msg_ids FROM ai_jobs WHERE is_batch = TRUE;"
```

---

## Next Steps

Want me to implement:
1. ✅ **Chat History Integration** (Phase 1)?
2. ⏭️ Message Batching (Phase 2)?
3. ⏭️ Both?

Let me know and I'll create the code!
