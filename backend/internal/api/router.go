package api

import (
	"net/http"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/handlers"
)

func newRouter(deps *handlers.Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("POST /photo/analyze", handlers.NewAnalyzeHandler(deps))
	mux.Handle("POST /photo/chat", handlers.NewChatHandler(deps))

	return mux
}
