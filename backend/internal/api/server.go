package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"google.golang.org/adk/session"
	"google.golang.org/adk/session/vertexai"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/agent"
	"github.com/matsuvr/photo_levelup_agent/backend/internal/handlers"
	firestoreSession "github.com/matsuvr/photo_levelup_agent/backend/internal/session"
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
	agentEngineID := os.Getenv("AGENT_ENGINE_ID")

	var sessionService session.Service

	// Priority: Vertex AI Session Service > Firestore > In-Memory
	if projectID != "" && location != "" && agentEngineID != "" {
		// Use Vertex AI Session Service (Agent Engine) for production
		log.Printf("Initializing Vertex AI Session Service for project: %s, location: %s, agent engine: %s", projectID, location, agentEngineID)

		// Build the full reasoning engine resource name
		reasoningEngine := fmt.Sprintf("projects/%s/locations/%s/reasoningEngines/%s", projectID, location, agentEngineID)

		sessionService, err = vertexai.NewSessionService(ctx, vertexai.VertexAIServiceConfig{
			ProjectID:       projectID,
			Location:        location,
			ReasoningEngine: reasoningEngine,
		})
		if err != nil {
			log.Printf("Warning: Failed to create Vertex AI session service: %v. Falling back to Firestore.", err)
			sessionService = nil // Will try Firestore next
		} else {
			log.Println("Vertex AI Session Service initialized successfully")
		}
	}

	// Fallback to Firestore if Vertex AI Session Service is not available
	if sessionService == nil && projectID != "" {
		log.Printf("Initializing Firestore session service for project: %s", projectID)
		sessionService, err = firestoreSession.NewFirestoreService(ctx, projectID)
		if err != nil {
			log.Printf("Warning: Failed to create Firestore session service: %v. Falling back to in-memory.", err)
			sessionService = session.InMemoryService()
		} else {
			log.Println("Firestore session service initialized successfully")
		}
	}

	// Final fallback to in-memory
	if sessionService == nil {
		log.Println("GOOGLE_CLOUD_PROJECT not set. Using in-memory session service.")
		sessionService = session.InMemoryService()
	}

	deps := handlers.NewDependencies(photoAgent, sessionService)
	router := newRouter(deps)

	return &Server{router: router}, nil
}

func (s *Server) Handler() http.Handler {
	return s.router
}
