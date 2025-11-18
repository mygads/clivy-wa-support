package services

import (
	"fmt"
	"log"
	"strings"

	"genfity-wa-support/database"
	"genfity-wa-support/models"
)

// ContextData holds system prompt and user message for LLM
type ContextData struct {
	SystemPrompt string
	UserMessage  string
}

// Document represents knowledge base document
type Document struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Kind    string `json:"kind"`
}

// BotSettings holds bot configuration from transactional DB
type BotSettings struct {
	SystemPrompt string     `json:"systemPrompt"`
	FallbackText string     `json:"fallbackText"`
	Documents    []Document `json:"documents"`
}

// BuildContext fetches bot settings and builds context for LLM with default limit (10 messages)
func BuildContext(userID, sessionToken, messageID string) (*ContextData, error) {
	return BuildContextWithLimit(userID, sessionToken, messageID, 10)
}

// BuildContextWithLimit builds context with dynamic message limit
func BuildContextWithLimit(userID, sessionToken, messageID string, maxMessages int) (*ContextData, error) {
	// 1. Fetch bot settings using data provider (respects DATA_ACCESS_MODE env)
	provider, err := GetDataProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to get data provider: %w", err)
	}

	botSettings, err := provider.GetBotSettings(userID, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bot settings: %w", err)
	}

	// 2. Get current message first (needed for smart doc filtering)
	db := database.GetDB()
	var currentMsg models.AIChatMessage
	err = db.Where("message_id = ?", messageID).First(&currentMsg).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch current message: %w", err)
	}

	// 3. Fetch chat history with dynamic limit
	var messages []models.AIChatMessage
	err = db.Where("session_tok = ?", sessionToken).
		Order("timestamp DESC").
		Limit(maxMessages).
		Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chat history: %w", err)
	}

	// 4. Build system prompt
	systemPrompt := botSettings.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "Anda adalah customer service yang ramah dan profesional."
		log.Printf("⚠️  Using default system prompt (no custom prompt found)")
	} else {
		log.Printf("✅ Using custom system prompt: %d chars", len(systemPrompt))
		// Show first 300 chars of system prompt for debugging
		previewLen := min(300, len(systemPrompt))
		log.Printf("📝 System prompt preview: %s...", systemPrompt[:previewLen])
	}

	// Add WhatsApp formatting instructions to system prompt
	systemPrompt += `

=== PENTING: Format Pesan WhatsApp ===
Gunakan format WhatsApp yang benar:
- Untuk BOLD gunakan *teks* (1 asterisk di kiri dan kanan)
- Untuk ITALIC gunakan _teks_ (1 underscore)
- Untuk list/bullet gunakan tanda hubung: - item
- JANGAN gunakan **teks** (double asterisk markdown)
- JANGAN gunakan * di awal kalimat untuk list

Contoh format yang BENAR:
- *Pengembangan Website:* Membuat situs web profesional
- *Pengembangan Aplikasi:* Merancang aplikasi mobile
- *Branding:* Membangun identitas merek

Contoh format yang SALAH:
*   **Pengembangan Website:** Membuat situs web
* Item list

=== ATURAN KOMUNIKASI ===
1. JANGAN mengulang sapaan "Halo!" di setiap pesan - cukup sekali di awal percakapan
2. JANGAN ulangi penjelasan yang sudah diberikan sebelumnya
3. Gunakan conversation history untuk memberikan jawaban yang kontekstual
4. Jika user tanya harga/estimasi:
   - Berikan range harga yang realistis (contoh: "Rp 5-10 juta" atau "mulai dari Rp 3 juta")
   - Jelaskan faktor yang mempengaruhi harga
   - Tawarkan konsultasi gratis untuk estimasi detail
5. Jika user sudah jelaskan kebutuhan, langsung berikan rekomendasi konkret
6. Gunakan bahasa natural seperti chat WhatsApp biasa, bukan email formal
7. Fokus pada solusi dan action items, bukan mengulang pertanyaan
8. Jika sudah ada context dari chat sebelumnya, langsung lanjutkan - jangan reset percakapan

Contoh BURUK (jangan ditiru):
"Halo! Kami dapat membantu Anda. Untuk memberikan estimasi yang akurat, bisakah Anda jelaskan..."

Contoh BAIK (ikuti ini):
"Untuk website e-commerce dengan fitur yang Anda sebutkan (landing page + order + payment), estimasi biaya sekitar Rp 8-12 juta tergantung kompleksitas payment gateway. Sudah termasuk desain UI/UX dan integrasi API. Mau saya buatkan breakdown detailnya?"
`

	// Add knowledge base with smart selection based on user query
	// For better context relevance, we can filter docs based on keywords in the current message
	relevantDocs := botSettings.Documents

	// If there are many documents, try to prioritize relevant ones
	if len(botSettings.Documents) > 10 {
		log.Printf("📚 Large knowledge base detected (%d docs), applying smart filtering...", len(botSettings.Documents))
		relevantDocs = filterRelevantDocuments(botSettings.Documents, currentMsg.Body)
		log.Printf("✅ Filtered to %d relevant documents", len(relevantDocs))
	}

	// Limit to top documents to avoid context overflow
	knowledgeLimit := 10 // Increased from 5 to 10
	if len(relevantDocs) > knowledgeLimit {
		log.Printf("⚠️  Limiting knowledge base to %d docs (total: %d)",
			knowledgeLimit, len(relevantDocs))
		relevantDocs = relevantDocs[:knowledgeLimit]
	}

	if len(relevantDocs) > 0 {
		systemPrompt += "\n\n=== Knowledge Base - WAJIB DIGUNAKAN ===\n"
		systemPrompt += "ATURAN PENTING:\n"
		systemPrompt += "1. SELALU gunakan informasi dari knowledge base untuk menjawab pertanyaan tentang harga, layanan, dan fitur\n"
		systemPrompt += "2. Jangan membuat estimasi harga sendiri - gunakan HARGA PASTI dari knowledge base\n"
		systemPrompt += "3. Jika user tanya harga, sebutkan paket yang relevan dengan ANGKA PASTI\n"
		systemPrompt += "4. Jika knowledge base tidak memiliki info yang ditanya, baru boleh minta detail atau tawarkan konsultasi\n\n"

		for _, doc := range relevantDocs {
			// Dynamic limit based on document type
			maxLength := 5000 // Increased from 3000 for pricing docs
			if doc.Kind == "pricing" || doc.Kind == "price" {
				maxLength = 8000 // Even higher for pricing - most important!
			}

			content := doc.Content
			originalLength := len(content)
			if len(content) > maxLength {
				content = content[:maxLength] + "..."
				log.Printf("⚠️  Document '%s' truncated to %d chars (original: %d)", doc.Title, maxLength, originalLength)
			}
			systemPrompt += fmt.Sprintf("\n[%s - %s]\n%s\n", doc.Kind, doc.Title, content)
		}
		systemPrompt += "\n--- End of Knowledge Base ---\n"
	}

	// Add chat history
	if len(messages) > 0 {
		systemPrompt += "\n\n=== Conversation History ===\n"
		systemPrompt += "PENTING: Gunakan percakapan di bawah untuk memahami konteks dan JANGAN ulangi informasi yang sudah diberikan.\n\n"
		// Reverse order (oldest first)
		for i := len(messages) - 1; i >= 0; i-- {
			msg := messages[i]
			role := "Customer"
			if msg.FromMe {
				role = "Assistant"
			}
			// Limit message body to 200 characters
			body := msg.Body
			if len(body) > 200 {
				body = body[:200] + "..."
			}
			systemPrompt += fmt.Sprintf("%s: %s\n", role, body)
		}
		systemPrompt += "\n--- End of History ---\n"
		systemPrompt += "Sekarang lanjutkan percakapan dengan natural berdasarkan context di atas. Jangan reset atau ulangi info yang sudah dijelaskan.\n"
	}

	// Add final reminder about knowledge base
	systemPrompt += "\n\n=== REMINDER SEBELUM MENJAWAB ===\n"
	systemPrompt += "Sebelum menjawab pertanyaan user:\n"
	systemPrompt += "1. CEK KNOWLEDGE BASE terlebih dahulu - terutama untuk pertanyaan harga/layanan\n"
	systemPrompt += "2. Gunakan HARGA PASTI dari knowledge base, jangan buat estimasi sendiri\n"
	systemPrompt += "3. Sebutkan nama paket yang sesuai (Starter/Business/Prime/Enterprise)\n"
	systemPrompt += "4. Jika knowledge base tidak cukup, baru tawarkan konsultasi detail\n"

	// Estimate token count (rough: 1 token ≈ 4 chars)
	estimatedTokens := (len(systemPrompt) + len(currentMsg.Body)) / 4
	log.Printf("📊 Context size: ~%d tokens (system: %d chars, user: %d chars, messages: %d)",
		estimatedTokens, len(systemPrompt), len(currentMsg.Body), maxMessages)

	return &ContextData{
		SystemPrompt: systemPrompt,
		UserMessage:  currentMsg.Body,
	}, nil
}

// filterRelevantDocuments filters documents based on keyword relevance to user query
// Returns documents sorted by relevance score (highest first)
func filterRelevantDocuments(docs []Document, userQuery string) []Document {
	if len(docs) == 0 {
		return docs
	}

	// Normalize query to lowercase for matching
	query := strings.ToLower(userQuery)

	// Define keyword categories and their weights
	keywords := map[string][]string{
		"pricing":  {"harga", "biaya", "price", "cost", "berapa", "paket", "rp", "rupiah", "juta", "ribu"},
		"whatsapp": {"whatsapp", "wa", "api", "chat", "pesan", "message"},
		"website":  {"website", "web", "landing", "page", "situs", "company profile", "ecommerce", "e-commerce"},
		"seo":      {"seo", "search", "google", "optimization", "ranking"},
		"app":      {"aplikasi", "app", "mobile", "android", "ios"},
		"general":  {"layanan", "service", "genfity", "bantuan", "help"},
	}

	type scoredDoc struct {
		doc   Document
		score int
	}

	scored := make([]scoredDoc, 0, len(docs))

	for _, doc := range docs {
		docContent := strings.ToLower(doc.Title + " " + doc.Content + " " + doc.Kind)
		score := 0

		// Score based on keyword matches
		for category, words := range keywords {
			for _, keyword := range words {
				// Check if keyword in query
				if strings.Contains(query, keyword) {
					// Check if keyword also in document
					if strings.Contains(docContent, keyword) {
						// Pricing gets highest weight
						if category == "pricing" {
							score += 10
						} else {
							score += 5
						}
					}
				}
			}
		}

		// Boost score for pricing documents if query contains price-related keywords
		if doc.Kind == "pricing" || doc.Kind == "price" {
			for _, keyword := range keywords["pricing"] {
				if strings.Contains(query, keyword) {
					score += 15 // Extra boost for pricing docs
					break
				}
			}
		}

		// Always include pricing docs with at least base score if query is about pricing
		if doc.Kind == "pricing" || doc.Kind == "price" {
			if score == 0 {
				for _, keyword := range keywords["pricing"] {
					if strings.Contains(query, keyword) {
						score = 5 // Minimum score for pricing docs
						break
					}
				}
			}
		}

		scored = append(scored, scoredDoc{doc: doc, score: score})
	}

	// Sort by score (highest first)
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Return sorted documents
	result := make([]Document, len(scored))
	for i, sd := range scored {
		result[i] = sd.doc
	}

	// Log top 3 documents for debugging
	if len(result) > 0 {
		log.Printf("📄 Top relevant docs: ")
		for i := 0; i < min(3, len(scored)); i++ {
			log.Printf("   %d. [%s] %s (score: %d)",
				i+1, scored[i].doc.Kind, scored[i].doc.Title, scored[i].score)
		}
	}

	return result
}
