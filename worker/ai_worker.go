package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/models"
	"genfity-wa-support/services"

	"github.com/lib/pq"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/gorm"
)

// Global circuit breaker for OpenRouter
var openRouterCB = services.NewCircuitBreaker("openrouter", 5, 60*time.Second)

// AIWorker processes AI jobs from queue
type AIWorker struct {
	llmClient *openai.Client
	db        *gorm.DB
	listener  *pq.Listener
	shutdown  chan struct{}
	wg        sync.WaitGroup
}

// NewAIWorker creates new AI worker instance
func NewAIWorker() *AIWorker {
	return &AIWorker{
		llmClient: services.NewOpenRouterClient(),
		db:        database.GetDB(),
		shutdown:  make(chan struct{}),
	}
}

// Start begins the AI worker loop
func (w *AIWorker) Start() {
	log.Println("🤖 AI Worker started")

	// Setup LISTEN for real-time notifications
	w.wg.Add(1)
	go w.listenForJobs()

	// Fallback polling (every 2 seconds if no notifications)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.shutdown:
			log.Println("🛑 AI Worker shutting down...")
			w.wg.Wait() // Wait for listener to finish
			log.Println("✅ AI Worker stopped")
			return
		case <-ticker.C:
			w.processJobs()
		}
	}
}

// Stop signals worker to shutdown gracefully
func (w *AIWorker) Stop() {
	close(w.shutdown)
}

// listenForJobs sets up PostgreSQL LISTEN for job notifications with auto-reconnect
func (w *AIWorker) listenForJobs() {
	defer w.wg.Done()

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	// Callback untuk handle connection events (reconnection)
	// Cloud PostgreSQL (Prisma Cloud) aggressively closes LISTEN connections
	// This is expected behavior - polling fallback (2s) ensures jobs are processed
	eventCallback := func(ev pq.ListenerEventType, err error) {
		switch ev {
		case pq.ListenerEventConnected:
			log.Println("✅ [LISTEN] Connected - instant notifications enabled")
		case pq.ListenerEventDisconnected:
			// Silent - cloud DB will disconnect frequently, polling handles it
			log.Println("ℹ️  [LISTEN] Disconnected (polling fallback active)")
		case pq.ListenerEventReconnected:
			log.Println("✅ [LISTEN] Reconnected")
		case pq.ListenerEventConnectionAttemptFailed:
			// Only log non-connection errors - connection failures are expected on cloud DB
			if err != nil && !strings.Contains(err.Error(), "connection") && !strings.Contains(err.Error(), "forcibly closed") {
				log.Printf("⚠️  [LISTEN] Error: %v (polling fallback active)\n", err)
			}
			// Else: silent - this is normal for cloud PostgreSQL
		}
	}

	// Create listener with auto-reconnect:
	// - minReconnectInterval: 10s (wait 10s before first reconnect attempt)
	// - maxReconnectInterval: 1min (max wait between reconnect attempts)
	listener := pq.NewListener(connStr, 10*time.Second, time.Minute, eventCallback)

	err := listener.Listen("ai_jobs_channel")
	if err != nil {
		log.Fatalf("Failed to listen on ai_jobs_channel: %v", err)
	}
	defer listener.Close()

	log.Println("👂 Listening for AI job notifications on ai_jobs_channel...")

	// Keepalive ticker - ping every 60 seconds
	keepaliveTicker := time.NewTicker(60 * time.Second)
	defer keepaliveTicker.Stop()

	for {
		select {
		case <-w.shutdown:
			log.Println("🔕 Stopping job listener...")
			return

		case notification := <-listener.Notify:
			if notification != nil {
				log.Println("⚡ [LISTEN] Instant notification - processing jobs")
				w.processJobs()
			}
			// notification == nil means connection was lost and reconnected
			// pq.Listener will handle reconnection automatically

		case <-keepaliveTicker.C:
			// Send ping to keep connection alive (cloud DB will still disconnect)
			go func() {
				_ = listener.Ping() // Silent - ping failures are expected on cloud DB
			}()
		}
	}
}

// processJobs fetches and processes pending jobs with row locking
func (w *AIWorker) processJobs() {
	for {
		// Lock & fetch one job (FOR UPDATE SKIP LOCKED prevents race conditions)
		var job models.AIJob
		tx := w.db.Begin()

		err := tx.Raw(`
			SELECT * FROM ai_jobs
			WHERE status = 'pending'
			AND (next_run_at IS NULL OR next_run_at <= NOW())
			ORDER BY priority ASC, id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		`).Scan(&job).Error

		if err != nil || job.ID == 0 {
			tx.Rollback()
			return // No jobs available
		}

		// Update status to processing
		tx.Model(&job).Updates(map[string]interface{}{
			"status":     "processing",
			"attempts":   job.Attempts + 1,
			"updated_at": time.Now(),
		})
		tx.Commit()

		// Process the job (blocking)
		w.processJob(&job)
	}
}

// processJob executes single AI job
func (w *AIWorker) processJob(job *models.AIJob) {
	log.Printf("⚙️  Processing job #%d (message: %s, attempt: %d)", job.ID, job.MessageID, job.Attempts)

	start := time.Now()

	// Create job attempt record
	attempt := models.AIJobAttempt{
		JobID:     job.ID,
		StartedAt: start,
		Status:    "processing",
	}
	w.db.Create(&attempt)

	// Get sender phone from chat message (we'll need this for typing indicator and auto-read)
	var chatMsg models.AIChatMessage
	err := w.db.Where("message_id = ?", job.MessageID).First(&chatMsg).Error
	if err != nil {
		w.failJob(job, &attempt, fmt.Sprintf("Failed to fetch chat message: %v", err))
		return
	}

	// ASYNC: Auto-read ALL unread messages for this contact (AI bot feature - always enabled)
	go func(sessionToken, senderPhone string) {
		// Get all unread incoming messages for this session+sender
		unreadMessages, err := services.GetUnreadIncomingMessages(sessionToken, senderPhone)
		if err != nil {
			log.Printf("⚠️  [AI Worker] Failed to get unread messages: %v", err)
			return
		}

		if len(unreadMessages) == 0 {
			return // No unread messages
		}

		// Extract message IDs
		messageIDs := make([]string, len(unreadMessages))
		for i, msg := range unreadMessages {
			messageIDs[i] = msg.MessageID
		}

		// Clean phone number (remove @s.whatsapp.net suffix)
		phoneNumber := strings.TrimSuffix(senderPhone, "@s.whatsapp.net")

		log.Printf("📖 [AI Bot] Auto-reading %d unread messages for contact %s", len(messageIDs), phoneNumber)

		// Call WA Server to mark as read
		if err := services.MarkMessagesAsRead(sessionToken, messageIDs, phoneNumber); err != nil {
			log.Printf("⚠️  [AI Bot] Failed to mark messages as read via WA Server: %v", err)
			// Continue even if markread fails
		}

		// Update DB
		if err := services.MarkMessagesAsReadInDB(messageIDs); err != nil {
			log.Printf("⚠️  [AI Bot] Failed to mark messages as read in DB: %v", err)
		}
	}(job.SessionTok, chatMsg.From)

	// 1. Build context (fetch bot settings + chat history)
	maxMessages := 10
	ctx, err := services.BuildContextWithLimit(job.UserID, job.SessionTok, job.MessageID, maxMessages)
	if err != nil {
		w.failJob(job, &attempt, fmt.Sprintf("Context build failed: %v", err))
		return
	}

	// Log system prompt preview for debugging
	log.Printf("🤖 System prompt to LLM (first 400 chars): %s...", ctx.SystemPrompt[:min(400, len(ctx.SystemPrompt))])
	log.Printf("💬 User message to LLM: %s", ctx.UserMessage)

	// AI BOT: Show typing indicator BEFORE calling LLM (always enabled for AI)
	phoneNumber := strings.TrimSuffix(chatMsg.From, "@s.whatsapp.net")
	if err := services.SetTypingState(job.SessionTok, phoneNumber, "composing"); err != nil {
		log.Printf("⚠️  [AI Bot] Failed to set typing state to composing: %v", err)
		// Continue even if typing indicator fails
	}

	// 2. Call LLM with timeout and circuit breaker
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var response string
	var inTok, outTok int

	// Use circuit breaker to prevent cascading failures
	cbErr := openRouterCB.Call(func() error {
		var llmErr error
		response, inTok, outTok, llmErr = services.AskLLM(timeoutCtx, w.llmClient, ctx.SystemPrompt, ctx.UserMessage)
		return llmErr
	})

	if cbErr != nil {
		// Stop typing indicator on error
		services.SetTypingState(job.SessionTok, phoneNumber, "stop")

		// Parse error for intelligent handling
		w.handleLLMError(job, &attempt, cbErr, maxMessages)
		return
	}

	// AI BOT: Stop typing indicator AFTER LLM responds, BEFORE sending message
	if err := services.SetTypingState(job.SessionTok, phoneNumber, "stop"); err != nil {
		log.Printf("⚠️  [AI Bot] Failed to set typing state to stop: %v", err)
	}

	latency := time.Since(start).Milliseconds()

	// 3. Sender info already fetched earlier (chatMsg variable)

	// 4. Send reply via WA (using internal gateway)
	err = services.SendWAText(job.SessionTok, chatMsg.From, response)
	if err != nil {
		w.failJob(job, &attempt, fmt.Sprintf("Failed to send WA message: %v", err))
		return
	}

	// 4b. Save AI response to AI chat history (for context builder) AND permanent chat history
	go func() {
		// Save to ai_chat_messages (for AI context) with FromMe=true, IsRead=true
		// We don't have the actual message ID from WA server, so use a generated one
		aiMsgID := fmt.Sprintf("ai_%s_%d", job.SessionTok, time.Now().UnixNano())
		if err := services.SaveOutgoingMessageToAIChat(
			job.SessionTok,
			aiMsgID,
			chatMsg.To,   // from (bot's JID)
			chatMsg.From, // to (recipient)
			response,     // body
			time.Now(),   // timestamp
		); err != nil {
			log.Printf("⚠️  Failed to save AI response to AI chat messages: %v", err)
		}

		// Save to permanent chat_messages (for UI)
		if err := services.SaveAIResponseToHistory(job.SessionTok, chatMsg.From, response); err != nil {
			log.Printf("⚠️  Failed to save AI response to permanent chat history: %v", err)
		}
	}()

	// 5. Log sent message
	sendLog := models.MessageSendLog{
		SessionTok: job.SessionTok,
		To:         chatMsg.From,
		Body:       response,
		Status:     "sent",
		CreatedAt:  time.Now(),
	}
	w.db.Create(&sendLog)

	// 6. Save AI output & mark job as done
	outputData := map[string]interface{}{
		"response":      response,
		"input_tokens":  inTok,
		"output_tokens": outTok,
		"latency_ms":    latency,
	}
	outputJSON, _ := json.Marshal(outputData)

	now := time.Now()
	w.db.Model(job).Updates(map[string]interface{}{
		"status":      "done",
		"output_json": string(outputJSON),
		"updated_at":  now,
	})

	// Update attempt record
	w.db.Model(&attempt).Updates(map[string]interface{}{
		"status":   "ok",
		"ended_at": now,
	})

	log.Printf("✅ Job #%d completed in %dms (tokens: %d in, %d out)",
		job.ID, latency, inTok, outTok)

	// Log to Transactional DB (AIUsageLog) - async, don't block on error
	go w.logUsage(job.UserID, job.SessionTok, inTok, outTok, int(latency), "ok", "")
}

// handleLLMError handles LLM errors with intelligent retry logic
func (w *AIWorker) handleLLMError(job *models.AIJob, attempt *models.AIJobAttempt, err error, currentMaxMessages int) {
	log.Printf("🔍 Analyzing error for job #%d: %v", job.ID, err)

	// Parse as OpenRouter error
	orErr := services.ParseSDKError(err)

	// Check if it's a context length error and we can retry with smaller context
	if orErr.IsContextLengthError() && currentMaxMessages > 5 {
		log.Printf("📏 Context too long, retrying job #%d with 5 messages instead of %d", job.ID, currentMaxMessages)

		// Retry with smaller context
		start := time.Now()
		smallerCtx, ctxErr := services.BuildContextWithLimit(job.UserID, job.SessionTok, job.MessageID, 5)
		if ctxErr != nil {
			w.permanentFailJob(job, attempt, fmt.Sprintf("Context build failed even with 5 messages: %v", ctxErr))
			return
		}

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		var response string
		var inTok, outTok int

		cbErr := openRouterCB.Call(func() error {
			var llmErr error
			response, inTok, outTok, llmErr = services.AskLLM(timeoutCtx, w.llmClient, smallerCtx.SystemPrompt, smallerCtx.UserMessage)
			return llmErr
		})

		if cbErr != nil {
			// Still failed, handle normally
			w.handleLLMError(job, attempt, cbErr, 5)
			return
		}

		// Success! Complete the job
		latency := time.Since(start).Milliseconds()

		var chatMsg models.AIChatMessage
		if err := w.db.Where("message_id = ?", job.MessageID).First(&chatMsg).Error; err != nil {
			w.failJob(job, attempt, fmt.Sprintf("Failed to fetch chat message: %v", err))
			return
		}

		if err := services.SendWAText(job.SessionTok, chatMsg.From, response); err != nil {
			w.failJob(job, attempt, fmt.Sprintf("Failed to send WA message: %v", err))
			return
		}

		// Save AI response to ai_chat_messages AND permanent history
		go func(sessionToken, recipientJID, responseText string) {
			// Save to ai_chat_messages
			aiMsgID := fmt.Sprintf("ai_%s_%d", sessionToken, time.Now().UnixNano())
			if err := services.SaveOutgoingMessageToAIChat(
				sessionToken,
				aiMsgID,
				chatMsg.To,
				recipientJID,
				responseText,
				time.Now(),
			); err != nil {
				log.Printf("⚠️  Failed to save AI response to AI chat messages: %v", err)
			}

			// Save to permanent chat_messages
			if err := services.SaveAIResponseToHistory(sessionToken, recipientJID, responseText); err != nil {
				log.Printf("⚠️  Failed to save AI response to permanent chat history: %v", err)
			}
		}(job.SessionTok, chatMsg.From, response)

		sendLog := models.MessageSendLog{
			SessionTok: job.SessionTok,
			To:         chatMsg.From,
			Body:       response,
			Status:     "sent",
			CreatedAt:  time.Now(),
		}
		w.db.Create(&sendLog)

		outputData := map[string]interface{}{
			"response":      response,
			"input_tokens":  inTok,
			"output_tokens": outTok,
			"latency_ms":    latency,
		}
		outputJSON, _ := json.Marshal(outputData)

		now := time.Now()
		w.db.Model(job).Updates(map[string]interface{}{
			"status":      "done",
			"output_json": string(outputJSON),
			"updated_at":  now,
		})

		w.db.Model(attempt).Updates(map[string]interface{}{
			"status":   "ok",
			"ended_at": now,
		})

		log.Printf("✅ Job #%d completed with smaller context in %dms (tokens: %d in, %d out)",
			job.ID, latency, inTok, outTok)

		go w.logUsage(job.UserID, job.SessionTok, inTok, outTok, int(latency), "ok", "")
		return
	}

	// Check if error is permanent (non-retryable)
	if orErr.IsAuthError() || orErr.IsPaymentError() || orErr.IsModerationError() {
		w.permanentFailJob(job, attempt, fmt.Sprintf("%d: %s", orErr.Code, orErr.Message))
		return
	}

	// Check if we should retry
	if !orErr.IsRetryable() {
		w.permanentFailJob(job, attempt, fmt.Sprintf("Non-retryable error: %d - %s", orErr.Code, orErr.Message))
		return
	}

	// Retryable error - use normal retry logic
	errMsg := fmt.Sprintf("LLM call failed (%d): %s", orErr.Code, orErr.Message)
	w.failJob(job, attempt, errMsg)
}

// permanentFailJob marks job as permanently failed (no retry)
func (w *AIWorker) permanentFailJob(job *models.AIJob, attempt *models.AIJobAttempt, errMsg string) {
	log.Printf("🚫 Job #%d permanently failed: %s", job.ID, errMsg)

	now := time.Now()

	// Update attempt
	w.db.Model(attempt).Updates(map[string]interface{}{
		"status":    "error",
		"ended_at":  now,
		"error_msg": errMsg,
	})

	// Mark job as failed (no retry)
	w.db.Model(job).Updates(map[string]interface{}{
		"status":     "failed",
		"error_msg":  errMsg,
		"updated_at": now,
	})

	// Log to usage with error status
	go w.logUsage(job.UserID, job.SessionTok, 0, 0, 0, "error", errMsg)
}

// failJob marks job as failed with retry logic
func (w *AIWorker) failJob(job *models.AIJob, attempt *models.AIJobAttempt, errMsg string) {
	log.Printf("❌ Job #%d failed: %s", job.ID, errMsg)

	now := time.Now()

	// Update attempt record
	w.db.Model(attempt).Updates(map[string]interface{}{
		"status":    "error",
		"error_msg": errMsg,
		"ended_at":  now,
	})

	updates := map[string]interface{}{
		"error_msg":  errMsg,
		"updated_at": now,
	}

	// Retry logic (max 3 attempts)
	if job.Attempts < 3 {
		nextRun := time.Now().Add(30 * time.Second)
		updates["status"] = "pending"
		updates["next_run_at"] = nextRun
		log.Printf("🔄 Job #%d will retry at %s (attempt %d/3)", job.ID, nextRun.Format(time.RFC3339), job.Attempts)
	} else {
		updates["status"] = "failed"
		log.Printf("💀 Job #%d permanently failed after %d attempts", job.ID, job.Attempts)

		// Log permanent failure to Transactional DB
		go w.logUsage(job.UserID, job.SessionTok, 0, 0, 0, "error", errMsg)
	}

	w.db.Model(job).Updates(updates)
}

// logUsage logs AI usage to Transactional DB via data provider (async)
func (w *AIWorker) logUsage(userID, sessionID string, inputTokens, outputTokens, latencyMs int, status, errorReason string) {
	// Get data provider
	provider, err := services.GetDataProvider()
	if err != nil {
		log.Printf("⚠️  Failed to get data provider for usage log: %v", err)
		return
	}

	// Prepare usage log request
	logReq := &services.UsageLogRequest{
		UserID:       userID,
		SessionID:    sessionID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		LatencyMs:    latencyMs,
		Status:       status,
		ErrorReason:  errorReason,
	}

	// Log usage via provider (API or Direct DB)
	if err := provider.LogUsage(logReq); err != nil {
		log.Printf("⚠️  Failed to log AI usage: %v", err)
	}
}
