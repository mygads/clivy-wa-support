package models

import "time"

// Transactional DB AI Models - MUST MATCH Prisma schema EXACTLY
// Source of truth: clivy-app/prisma/schema.prisma
// Migration: Prisma only (no GORM auto-migrate!)

// WhatsAppAIBot matches Prisma model WhatsAppAIBot
type WhatsAppAIBot struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	UserID       string    `gorm:"column:userId;not null" json:"userId"`
	Name         string    `gorm:"column:name;not null;default:'Default Bot'" json:"name"`
	IsActive     bool      `gorm:"column:isActive;not null;default:false" json:"isActive"`
	SystemPrompt *string   `gorm:"column:systemPrompt;type:text" json:"systemPrompt"`
	FallbackText *string   `gorm:"column:fallbackText;type:text" json:"fallbackText"`
	CreatedAt    time.Time `gorm:"column:createdAt;not null;default:now()" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"column:updatedAt;not null" json:"updatedAt"`
}

func (WhatsAppAIBot) TableName() string {
	return "WhatsAppAIBot"
}

// AIDocument matches Prisma model AIDocument
type AIDocument struct {
	ID          string    `gorm:"column:id;primaryKey" json:"id"`
	UserID      string    `gorm:"column:userId;not null" json:"userId"`
	Title       string    `gorm:"column:title;not null" json:"title"`
	Kind        string    `gorm:"column:kind;not null;default:'faq'" json:"kind"`
	Content     string    `gorm:"column:content;type:text;not null" json:"content"`
	EmbeddingID *string   `gorm:"column:embeddingId" json:"embeddingId"`
	IsActive    bool      `gorm:"column:isActive;not null;default:true" json:"isActive"`
	CreatedAt   time.Time `gorm:"column:createdAt;not null;default:now()" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updatedAt;not null" json:"updatedAt"`
}

func (AIDocument) TableName() string {
	return "AIDocument"
}

// AIUsageLog matches Prisma model AIUsageLog
type AIUsageLog struct {
	ID           string    `gorm:"column:id;primaryKey" json:"id"`
	UserID       string    `gorm:"column:userId;not null" json:"userId"`
	SessionID    *string   `gorm:"column:sessionId" json:"sessionId"`
	InputTokens  int       `gorm:"column:inputTokens;not null;default:0" json:"inputTokens"`
	OutputTokens int       `gorm:"column:outputTokens;not null;default:0" json:"outputTokens"`
	TotalTokens  int       `gorm:"column:totalTokens;not null;default:0" json:"totalTokens"`
	LatencyMs    int       `gorm:"column:latencyMs;not null;default:0" json:"latencyMs"`
	Status       string    `gorm:"column:status;not null;default:'ok'" json:"status"`
	ErrorReason  *string   `gorm:"column:errorReason;type:text" json:"errorReason"`
	CreatedAt    time.Time `gorm:"column:createdAt;not null;default:now()" json:"createdAt"`
}

func (AIUsageLog) TableName() string {
	return "AIUsageLog"
}

// AIBotSessionBinding matches Prisma model AIBotSessionBinding
type AIBotSessionBinding struct {
	ID        string    `gorm:"column:id;primaryKey" json:"id"`
	UserID    string    `gorm:"column:userId;not null" json:"userId"`
	BotID     string    `gorm:"column:botId;not null" json:"botId"`
	SessionID string    `gorm:"column:sessionId;unique;not null" json:"sessionId"`
	IsActive  bool      `gorm:"column:isActive;not null;default:true" json:"isActive"`
	CreatedAt time.Time `gorm:"column:createdAt;not null;default:now()" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updatedAt;not null" json:"updatedAt"`
}

func (AIBotSessionBinding) TableName() string {
	return "AIBotSessionBinding"
}
