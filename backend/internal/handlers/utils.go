package handlers

import "google.golang.org/genai"

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
