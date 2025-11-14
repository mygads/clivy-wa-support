package models

import "time"

// AIChatMessage: simpan semua pesan masuk/keluar untuk AI context
// Berbeda dengan ChatMessage yang sudah ada, ini khusus untuk AI processing
type AIChatMessage struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	MessageID  string    `gorm:"uniqueIndex;not null" json:"message_id"` // dari WA (Info.ID)
	SessionTok string    `gorm:"index;not null" json:"session_tok"`      // instanceName
	From       string    `gorm:"index;not null" json:"from"`             // nomor pengirim
	To         string    `gorm:"index;not null" json:"to"`               // nomor penerima
	FromMe     bool      `gorm:"default:false" json:"from_me"`           // aku yang kirim?
	MsgType    string    `gorm:"index;not null" json:"msg_type"`         // "text"
	Body       string    `gorm:"type:text" json:"body"`
	PushName   string    `json:"push_name"`
	IsRead     bool      `gorm:"default:false;index" json:"is_read"` // sudah di-read atau belum
	Timestamp  time.Time `gorm:"index" json:"timestamp"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName override untuk tabel ai_chat_messages
func (AIChatMessage) TableName() string {
	return "ai_chat_messages"
}

// MessageSendLog: hasil kirim balasan AI
type MessageSendLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SessionTok string    `gorm:"index;not null" json:"session_tok"`
	To         string    `gorm:"index;not null" json:"to"`
	Body       string    `gorm:"type:text" json:"body"`
	Status     string    `gorm:"index;default:'sent'" json:"status"` // sent|failed
	ErrorMsg   string    `gorm:"type:text" json:"error_msg"`
	CreatedAt  time.Time `json:"created_at"`
}

// AIJob: queue tanpa Redis
type AIJob struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Status     string     `gorm:"index;default:'pending'" json:"status"` // pending|processing|done|failed
	Priority   int        `gorm:"default:5" json:"priority"`
	SessionTok string     `gorm:"index;not null" json:"session_tok"`
	MessageID  string     `gorm:"index;not null" json:"message_id"`
	UserID     string     `gorm:"index;not null" json:"user_id"`
	InputJSON  string     `gorm:"type:text" json:"input_json"`  // payload ringkas (prompt, context keys)
	OutputJSON string     `gorm:"type:text" json:"output_json"` // jawaban LLM
	ErrorMsg   string     `gorm:"type:text" json:"error_msg"`
	Attempts   int        `gorm:"default:0" json:"attempts"`
	NextRunAt  *time.Time `gorm:"index" json:"next_run_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// AIJobAttempt: retry log
type AIJobAttempt struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	JobID     uint      `gorm:"index;not null" json:"job_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	Status    string    `json:"status"` // ok|timeout|error
	ErrorMsg  string    `gorm:"type:text" json:"error_msg"`
	CreatedAt time.Time `json:"created_at"`
}

// RateLimit: untuk rate limiting tanpa Redis
type RateLimit struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SessionTok  string    `gorm:"uniqueIndex;not null" json:"session_tok"`
	Counter     int       `gorm:"default:0" json:"counter"`
	WindowStart time.Time `gorm:"index" json:"window_start"`
	UpdatedAt   time.Time `json:"updated_at"`
}
