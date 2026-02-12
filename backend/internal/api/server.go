package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"google.golang.org/adk/session"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/agent"
	"github.com/matsuvr/photo_levelup_agent/backend/internal/auth"
	"github.com/matsuvr/photo_levelup_agent/backend/internal/handlers"
	"github.com/matsuvr/photo_levelup_agent/backend/internal/services"
	firestoreSession "github.com/matsuvr/photo_levelup_agent/backend/internal/session"
)

type Server struct {
	router        http.Handler
	storageClient *services.StorageClient
}

func NewServer(ctx context.Context) (*Server, error) {
	photoAgent, err := agent.NewPhotoCoachAgent(ctx)
	if err != nil {
		return nil, err
	}

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")

	var sessionService session.Service

	if projectID != "" {
		log.Printf("INFO: Initializing Firestore session service for project: %s", projectID)
		sessionService, err = firestoreSession.NewFirestoreService(ctx, projectID)
		if err != nil {
			log.Printf("WARN: Failed to create Firestore session service: %v. Falling back to in-memory.", err)
			sessionService = session.InMemoryService()
		} else {
			log.Println("INFO: Firestore session service initialized successfully")
		}
	} else {
		log.Println("INFO: GOOGLE_CLOUD_PROJECT not set. Using in-memory session service.")
		sessionService = session.InMemoryService()
	}

	firebaseAuth, err := auth.NewFirebaseAuthClient(ctx)
	if err != nil {
		log.Printf("WARN: Failed to initialize Firebase Auth: %v", err)
	}

	storageClient, err := services.NewStorageClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	geminiClient := services.NewGeminiClient()

	deps := handlers.NewDependencies(photoAgent, sessionService, storageClient, geminiClient)
	router := newRouter(deps, firebaseAuth)

	return &Server{
		router:        router,
		storageClient: storageClient,
	}, nil
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) Close() error {
	if s.storageClient != nil {
		return s.storageClient.Close()
	}
	return nil
}
