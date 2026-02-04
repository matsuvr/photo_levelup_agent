package api

import (
	"context"
	"log"
	"net/http"
	"os"

	"google.golang.org/adk/session"

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

	var sessionService session.Service

	// Use Firestore for session persistence if project ID is available
	if projectID != "" {
		log.Printf("Initializing Firestore session service for project: %s", projectID)
		sessionService, err = firestoreSession.NewFirestoreService(ctx, projectID)
		if err != nil {
			log.Printf("Warning: Failed to create Firestore session service: %v. Falling back to in-memory.", err)
			sessionService = session.InMemoryService()
		} else {
			log.Println("Firestore session service initialized successfully")
		}
	} else {
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
