package handlers

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/services"
)

type Dependencies struct {
	Agent          agent.Agent
	SessionService session.Service
	StorageClient  *services.StorageClient
	GeminiClient   *services.GeminiClient
}

func NewDependencies(
	agent agent.Agent,
	sessionService session.Service,
	storageClient *services.StorageClient,
	geminiClient *services.GeminiClient,
) *Dependencies {
	return &Dependencies{
		Agent:          agent,
		SessionService: sessionService,
		StorageClient:  storageClient,
		GeminiClient:   geminiClient,
	}
}
