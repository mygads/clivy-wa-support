package services

import (
	"regexp"
	"strings"
)

// FormatForWhatsApp converts markdown-style formatting to WhatsApp formatting
// WhatsApp supports:
// - *bold* (single asterisk or underscore on each side)
// - _italic_ (single underscore)
// - ~strikethrough~ (tilde)
// - ```code``` (triple backticks)
func FormatForWhatsApp(text string) string {
	// Step 1: Convert markdown bold (**text**) to WhatsApp bold (*text*)
	// Match **text** but not ***text*** (to avoid breaking already formatted text)
	reBold := regexp.MustCompile(`\*\*([^*]+?)\*\*`)
	text = reBold.ReplaceAllString(text, "*$1*")

	// Step 2: Convert markdown list items with bold
	// From: *   **Item:** description
	// To:   - *Item:* description
	reListBold := regexp.MustCompile(`(?m)^\*\s+\*([^*]+?)\*\s*(.*)$`)
	text = reListBold.ReplaceAllString(text, "- *$1* $2")

	// Step 3: Convert remaining markdown list items (* item) to WhatsApp style (- item)
	// Only convert at start of line with optional whitespace
	reList := regexp.MustCompile(`(?m)^\*\s+`)
	text = reList.ReplaceAllString(text, "- ")

	// Step 4: Clean up multiple consecutive newlines (max 2)
	reMultiNewline := regexp.MustCompile(`\n{3,}`)
	text = reMultiNewline.ReplaceAllString(text, "\n\n")

	// Step 5: Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	return text
}

// FormatBulletList converts markdown bullet points to WhatsApp-friendly format
// This is a specialized formatter for lists
func FormatBulletList(items []string, boldPrefix bool) string {
	var result strings.Builder

	for _, item := range items {
		// Split by colon if exists (e.g., "Title: description")
		parts := strings.SplitN(item, ":", 2)

		if len(parts) == 2 && boldPrefix {
			// Make the prefix bold
			title := strings.TrimSpace(parts[0])
			desc := strings.TrimSpace(parts[1])
			result.WriteString("- *")
			result.WriteString(title)
			result.WriteString(":* ")
			result.WriteString(desc)
		} else {
			// Simple bullet point
			result.WriteString("- ")
			result.WriteString(strings.TrimSpace(item))
		}
		result.WriteString("\n")
	}

	return strings.TrimSpace(result.String())
}

// StripMarkdown removes all markdown formatting
func StripMarkdown(text string) string {
	// Remove bold/italic markers
	text = regexp.MustCompile(`\*+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`_+`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`~+`).ReplaceAllString(text, "")
	text = regexp.MustCompile("`+").ReplaceAllString(text, "")

	return strings.TrimSpace(text)
}
