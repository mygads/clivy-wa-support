package services

import (
	"fmt"
	"log"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/models"

	"gorm.io/gorm"
)

const MaxMessagesPerContact = 20

// SaveToChatHistory saves incoming/outgoing messages to permanent chat history
// This is separate from ai_chat_messages which is temporary for AI context
func SaveToChatHistory(sessionToken, senderJID, recipientJID, body, pushName string, timestamp time.Time, fromMe bool) error {
	db := database.GetDB()

	// Determine chat participants
	var contactJID string
	if fromMe {
		contactJID = recipientJID // User sending to contact
	} else {
		contactJID = senderJID // Contact sending to user
	}

	// Create unique chat ID (session + contact)
	chatID := fmt.Sprintf("%s_%s", sessionToken, contactJID)

	// 1. Find or create ChatRoom
	var chatRoom models.ChatRoom
	err := db.Where("chat_id = ?", chatID).First(&chatRoom).Error

	if err == gorm.ErrRecordNotFound {
		// Create new chat room
		chatRoom = models.ChatRoom{
			ChatID:       chatID,
			UserToken:    sessionToken,
			ContactJID:   contactJID,
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
			log.Printf("❌ Failed to create chat room: %v", err)
			return fmt.Errorf("failed to create chat room: %w", err)
		}

		log.Printf("✅ Created new chat room: %s (contact: %s)", chatID, contactJID)
	} else if err != nil {
		log.Printf("❌ Failed to find chat room: %v", err)
		return fmt.Errorf("failed to find chat room: %w", err)
	} else {
		// Update existing chat room
		updates := map[string]interface{}{
			"last_message":  body,
			"last_sender":   getSenderType(fromMe),
			"last_activity": timestamp,
			"updated_at":    time.Now(),
		}

		// Increment unread count only for incoming messages
		if !fromMe {
			updates["unread_count"] = gorm.Expr("unread_count + ?", 1)
		}

		if err := db.Model(&chatRoom).Updates(updates).Error; err != nil {
			log.Printf("❌ Failed to update chat room: %v", err)
			return fmt.Errorf("failed to update chat room: %w", err)
		}

		log.Printf("✅ Updated chat room: %s (last_message: %.30s...)", chatID, body)
	}

	// 2. Save to ChatMessage
	// Generate message ID (or use WhatsApp message ID if available)
	messageID := fmt.Sprintf("%s_%d", chatID, time.Now().UnixNano())

	chatMessage := models.ChatMessage{
		MessageID:        messageID,
		ChatRoomID:       chatRoom.ID,
		ChatID:           chatID,
		UserToken:        sessionToken,
		SenderJID:        senderJID,
		SenderType:       getSenderType(fromMe),
		MessageType:      "text",
		Content:          body,
		Status:           "sent",
		MessageTimestamp: timestamp,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := db.Create(&chatMessage).Error; err != nil {
		log.Printf("❌ Failed to save chat message: %v", err)
		return fmt.Errorf("failed to save chat message: %w", err)
	}

	log.Printf("✅ Saved chat message to history: ID=%s, sender=%s, type=%s",
		messageID, getSenderType(fromMe), chatMessage.MessageType)

	return nil
}

// SaveAIResponseToHistory saves AI bot response to permanent chat history
func SaveAIResponseToHistory(sessionToken, recipientJID, response string) error {
	// AI bot sends message, so fromMe = true
	return SaveToChatHistory(
		sessionToken,
		sessionToken, // senderJID = session (bot)
		recipientJID, // recipientJID = contact
		response,     // body
		"AI Bot",     // pushName
		time.Now(),   // timestamp
		true,         // fromMe = true (bot is sending)
	)
}

// Helper functions

func getSenderType(fromMe bool) string {
	if fromMe {
		return "user"
	}
	return "contact"
}

func getUnreadIncrement(fromMe bool) int {
	if fromMe {
		return 0 // User's own messages don't increment unread
	}
	return 1 // Contact's messages increment unread
}

// ============= Auto Read Messages Functions =============

// SaveIncomingMessageToAIChat menyimpan pesan masuk ke ai_chat_messages dengan auto-cleanup
func SaveIncomingMessageToAIChat(sessionTok, messageID, from, to, body, pushName string, timestamp time.Time) error {
	db := database.GetDB()

	msg := models.AIChatMessage{
		MessageID:  messageID,
		SessionTok: sessionTok,
		From:       from,
		To:         to,
		FromMe:     false,
		MsgType:    "text",
		Body:       body,
		PushName:   pushName,
		IsRead:     false,
		Timestamp:  timestamp,
	}

	if err := db.Create(&msg).Error; err != nil {
		return fmt.Errorf("failed to save incoming message: %w", err)
	}

	// Cleanup: hapus pesan lama, keep only last 20 per SessionTok+From
	return CleanupOldAIChatMessages(sessionTok, from)
}

// SaveOutgoingMessageToAIChat menyimpan pesan keluar ke ai_chat_messages dengan auto-cleanup
func SaveOutgoingMessageToAIChat(sessionTok, messageID, from, to, body string, timestamp time.Time) error {
	db := database.GetDB()

	msg := models.AIChatMessage{
		MessageID:  messageID,
		SessionTok: sessionTok,
		From:       from,
		To:         to,
		FromMe:     true,
		MsgType:    "text",
		Body:       body,
		IsRead:     true, // outgoing message selalu dianggap sudah read
		Timestamp:  timestamp,
	}

	if err := db.Create(&msg).Error; err != nil {
		return fmt.Errorf("failed to save outgoing message: %w", err)
	}

	// Cleanup: hapus pesan lama, keep only last 20 per SessionTok+To
	return CleanupOldAIChatMessages(sessionTok, to)
}

// CleanupOldAIChatMessages hapus pesan lama, keep only last 20 per contact
func CleanupOldAIChatMessages(sessionTok, contactPhone string) error {
	db := database.GetDB()

	// Count total messages for this session+contact combination
	var count int64
	err := db.Model(&models.AIChatMessage{}).
		Where("session_tok = ? AND (\"from\" = ? OR \"to\" = ?)", sessionTok, contactPhone, contactPhone).
		Count(&count).Error

	if err != nil {
		return fmt.Errorf("failed to count messages: %w", err)
	}

	if count <= MaxMessagesPerContact {
		return nil // No cleanup needed
	}

	// Delete oldest messages, keeping only the last MaxMessagesPerContact
	toDelete := count - MaxMessagesPerContact

	// Get IDs of oldest messages to delete
	var oldMessageIDs []uint
	err = db.Model(&models.AIChatMessage{}).
		Where("session_tok = ? AND (\"from\" = ? OR \"to\" = ?)", sessionTok, contactPhone, contactPhone).
		Order("timestamp ASC").
		Limit(int(toDelete)).
		Pluck("id", &oldMessageIDs).Error

	if err != nil {
		return fmt.Errorf("failed to get old message IDs: %w", err)
	}

	if len(oldMessageIDs) > 0 {
		err = db.Where("id IN ?", oldMessageIDs).Delete(&models.AIChatMessage{}).Error
		if err != nil {
			return fmt.Errorf("failed to delete old messages: %w", err)
		}
		log.Printf("🧹 Cleaned up %d old messages for session %s, contact %s", len(oldMessageIDs), sessionTok, contactPhone)
	}

	return nil
}

// GetUnreadIncomingMessages ambil pesan incoming yang belum di-read untuk contact tertentu
func GetUnreadIncomingMessages(sessionTok, fromPhone string) ([]models.AIChatMessage, error) {
	db := database.GetDB()

	var messages []models.AIChatMessage
	err := db.
		Where("session_tok = ? AND \"from\" = ? AND from_me = ? AND is_read = ?", sessionTok, fromPhone, false, false).
		Order("timestamp ASC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get unread messages: %w", err)
	}

	return messages, nil
}

// MarkMessagesAsReadInDB update IsRead = true untuk message IDs tertentu
func MarkMessagesAsReadInDB(messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	db := database.GetDB()
	err := db.Model(&models.AIChatMessage{}).
		Where("message_id IN ?", messageIDs).
		Update("is_read", true).Error

	if err != nil {
		return fmt.Errorf("failed to mark messages as read in DB: %w", err)
	}

	log.Printf("✅ Marked %d messages as read in DB", len(messageIDs))
	return nil
}

// GetChatHistoryForAI ambil riwayat chat untuk AI context (compatibility dengan AI worker)
// Fungsi ini tetap digunakan oleh AI worker untuk build context
func GetChatHistoryForAI(sessionTok, contactPhone string, limit int) ([]models.AIChatMessage, error) {
	db := database.GetDB()

	var messages []models.AIChatMessage
	err := db.
		Where("session_tok = ? AND (\"from\" = ? OR \"to\" = ?)", sessionTok, contactPhone, contactPhone).
		Order("timestamp DESC").
		Limit(limit).
		Find(&messages).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to get chat history: %w", err)
	}

	// Reverse untuk urutan ascending (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}
