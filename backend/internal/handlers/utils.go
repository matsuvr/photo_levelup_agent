package handlers

import (
	"regexp"

	"google.golang.org/genai"
)

func extractText(content *genai.Content) string {
	if content == nil {
		return ""
	}

	text := ""
	for _, part := range content.Parts {
		if part == nil {
			continue
		}
		text += part.Text
	}

	return text
}

// fixMarkdownBold fixes markdown bold syntax by removing spaces between ** and text.
// Examples:
//   - "** text **" -> "**text**"
//   - "** text * text * text **" -> "**text * text * text**"
//   - "**text**" -> "**text**" (no change)
func fixMarkdownBold(text string) string {
	// Pattern: \*\* matches **, \s+ matches one or more spaces, (.+?) captures content (non-greedy), \s+ matches spaces, \*\* matches **
	// We need to handle the case where there might be single * inside double **
	re := regexp.MustCompile(`\*\*\s+(.+?)\s+\*\*`)
	return re.ReplaceAllString(text, `**$1**`)
}
