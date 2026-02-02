package api

import (
	"net/http"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/handlers"
)

func newRouter(deps *handlers.Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("POST /photo/analyze", handlers.NewAnalyzeHandler(deps))
	mux.Handle("GET /photo/analyze/status", handlers.NewAnalyzeStatusHandler())
	mux.Handle("POST /photo/chat", handlers.NewChatHandler(deps))
	mux.Handle("POST /test/gemini", handlers.NewTestGeminiHandler())

	return mux
}
