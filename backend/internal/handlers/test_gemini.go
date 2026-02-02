package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/services"
)

type TestGeminiHandler struct{}

func NewTestGeminiHandler() *TestGeminiHandler {
	return &TestGeminiHandler{}
}

type TestGeminiRequest struct {
	ImageURL string `json:"image_url"`
	Action   string `json:"action"` // "analyze" or "generate"
	Prompt   string `json:"prompt"`
}

func (h *TestGeminiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req TestGeminiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	geminiClient := services.NewGeminiClient()

	if req.Action == "generate" {
		if req.Prompt == "" {
			writeJSONError(w, http.StatusBadRequest, "prompt is required for generate action")
			return
		}
		result, err := geminiClient.GenerateImage(r.Context(), req.Prompt)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

	// Default: analyze
	if req.ImageURL == "" {
		writeJSONError(w, http.StatusBadRequest, "image_url is required")
		return
	}

	result, err := geminiClient.AnalyzeImage(r.Context(), req.ImageURL)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
