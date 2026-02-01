package api

import (
	"context"
	"net/http"
	"os"

	"google.golang.org/adk/session"
	"google.golang.org/adk/session/vertexai"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/agent"
	"github.com/matsuvr/photo_levelup_agent/backend/internal/handlers"
)

type Server struct {
	router http.Handler
}

func NewServer(ctx context.Context) (*Server, error) {
	photoAgent, err := agent.NewPhotoCoachAgent(ctx)
	if err != nil {
		return nil, err
	}

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	if location == "" {
		location = os.Getenv("GOOGLE_CLOUD_REGION")
	}
	agentEngineID := os.Getenv("AGENT_ENGINE_ID")

	var sessionService session.Service
	if projectID != "" && location != "" && agentEngineID != "" {
		sessionService, err = vertexai.NewSessionService(ctx, vertexai.VertexAIServiceConfig{
			ProjectID:       projectID,
			Location:        location,
			ReasoningEngine: agentEngineID,
		})
		if err != nil {
			return nil, err
		}
	} else {
		sessionService = session.InMemoryService()
	}

	deps := handlers.NewDependencies(photoAgent, sessionService)
	router := newRouter(deps)

	return &Server{router: router}, nil
}

func (s *Server) Handler() http.Handler {
	return s.router
}
