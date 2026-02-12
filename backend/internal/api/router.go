package api

import (
	"net/http"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/auth"
	"github.com/matsuvr/photo_levelup_agent/backend/internal/handlers"
	"github.com/matsuvr/photo_levelup_agent/backend/internal/middleware"
)

func newRouter(deps *handlers.Dependencies, firebaseAuth *auth.FirebaseAuthClient) http.Handler {
	mux := http.NewServeMux()

	authMiddleware := middleware.AuthMiddleware(firebaseAuth)

	mux.Handle("POST /photo/analyze", authMiddleware(handlers.NewAnalyzeHandler(deps)))
	mux.Handle("POST /photo/chat", authMiddleware(handlers.NewChatHandler(deps)))
	mux.Handle("GET /photo/sessions", authMiddleware(handlers.NewSessionsHandler(deps)))
	mux.Handle("GET /photo/sessions/", authMiddleware(handlers.NewSessionDetailHandler(deps)))

	mux.Handle("GET /photo/analyze/status", handlers.NewAnalyzeStatusHandler())
	mux.Handle("GET /photo/image", handlers.NewImageHandler())

	return mux
}
