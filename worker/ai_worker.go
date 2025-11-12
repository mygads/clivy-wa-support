package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/models"
	"genfity-wa-support/services"

	"github.com/lib/pq"
	openai "github.com/sashabaranov/go-openai"
	"gorm.io/gorm"
)

// AIWorker processes AI jobs from queue
type AIWorker struct {
	llmClient *openai.Client
	db        *gorm.DB
	listener  *pq.Listener
}

// NewAIWorker creates new AI worker instance
func NewAIWorker() *AIWorker {
	return &AIWorker{
		llmClient: services.NewOpenRouterClient(),
		db:        database.GetDB(),
	}
}

// Start begins the AI worker loop
func (w *AIWorker) Start() {
	log.Println("🤖 AI Worker started")

	// Setup LISTEN for real-time notifications
	go w.listenForJobs()

	// Fallback polling (every 2 seconds if no notifications)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		w.processJobs()
	}
}

// listenForJobs sets up PostgreSQL LISTEN for job notifications
func (w *AIWorker) listenForJobs() {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)

	listener := pq.NewListener(connStr, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Printf("Listener error: %v", err)
		}
	})

	err := listener.Listen("ai_jobs_channel")
	if err != nil {
		log.Fatalf("Failed to listen on ai_jobs_channel: %v", err)
	}

	log.Println("👂 Listening for AI job notifications on ai_jobs_channel...")

	for {
		select {
		case notification := <-listener.Notify:
			if notification != nil {
				log.Println("🔔 Job notification received")
				w.processJobs()
			}
		case <-time.After(90 * time.Second):
			// Ping to keep connection alive
			go listener.Ping()
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

	// 1. Build context (fetch bot settings + chat history)
	ctx, err := services.BuildContext(job.UserID, job.SessionTok, job.MessageID)
	if err != nil {
		w.failJob(job, &attempt, fmt.Sprintf("Context build failed: %v", err))
		return
	}

	// 2. Call LLM with timeout
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	response, inTok, outTok, err := services.AskLLM(timeoutCtx, w.llmClient, ctx.SystemPrompt, ctx.UserMessage)
	if err != nil {
		w.failJob(job, &attempt, fmt.Sprintf("LLM call failed: %v", err))
		return
	}

	latency := time.Since(start).Milliseconds()

	// 3. Get sender info from chat message
	var chatMsg models.AIChatMessage
	err = w.db.Where("message_id = ?", job.MessageID).First(&chatMsg).Error
	if err != nil {
		w.failJob(job, &attempt, fmt.Sprintf("Failed to fetch chat message: %v", err))
		return
	}

	// 4. Send reply via WA (using internal gateway)
	err = services.SendWAText(job.SessionTok, chatMsg.From, response)
	if err != nil {
		w.failJob(job, &attempt, fmt.Sprintf("Failed to send WA message: %v", err))
		return
	}

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

// logUsage logs AI usage to Transactional DB via API (async)
func (w *AIWorker) logUsage(userID, sessionID string, inputTokens, outputTokens, latencyMs int, status, errorReason string) {
	transactionalURL := os.Getenv("TRANSACTIONAL_API_URL")
	if transactionalURL == "" {
		transactionalURL = "http://localhost:8090/api"
	}

	apiKey := os.Getenv("INTERNAL_API_KEY")
	if apiKey == "" {
		log.Println("⚠️  INTERNAL_API_KEY not set, skipping usage log")
		return
	}

	url := fmt.Sprintf("%s/customer/ai/usage", transactionalURL)

	payload := map[string]interface{}{
		"userId":       userID,
		"sessionId":    sessionID,
		"inputTokens":  inputTokens,
		"outputTokens": outputTokens,
		"latencyMs":    latencyMs,
		"status":       status,
		"errorReason":  errorReason,
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("⚠️  Failed to create usage log request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️  Failed to log AI usage: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("⚠️  Usage log API returned %d: %s", resp.StatusCode, body)
	}
}
