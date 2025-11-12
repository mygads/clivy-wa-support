package services

import (
	"fmt"
	"log"
	"time"

	"genfity-wa-support/database"
	"genfity-wa-support/models"

	"gorm.io/gorm"
)

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
