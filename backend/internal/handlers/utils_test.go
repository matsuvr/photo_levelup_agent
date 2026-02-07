package handlers

import (
	"testing"
)

func TestFixMarkdownBold(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic case with spaces",
			input:    "** text **",
			expected: "**text**",
		},
		{
			name:     "Already correct",
			input:    "**text**",
			expected: "**text**",
		},
		{
			name:     "With single asterisks inside",
			input:    "** text * text * text **",
			expected: "**text * text * text**",
		},
		{
			name:     "Mixed text",
			input:    "normal text ** bold text ** normal",
			expected: "normal text **bold text** normal",
		},
		{
			name:     "Japanese text",
			input:    "選択された背景に対して、**「テクスチャ」と「明瞭度」**の数値をマイナスに振ります。",
			expected: "選択された背景に対して、**「テクスチャ」と「明瞭度」**の数値をマイナスに振ります。",
		},
		{
			name:     "Japanese with spaces",
			input:    "選択された背景に対して、** 「テクスチャ」と「明瞭度」 **の数値をマイナスに振ります。",
			expected: "選択された背景に対して、**「テクスチャ」と「明瞭度」**の数値をマイナスに振ります。",
		},
		{
			name:     "Multiple bold sections",
			input:    "** first ** and ** second **",
			expected: "**first** and **second**",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixMarkdownBold(tt.input)
			if result != tt.expected {
				t.Errorf("fixMarkdownBold() = %q, want %q", result, tt.expected)
			}
		})
	}
}
