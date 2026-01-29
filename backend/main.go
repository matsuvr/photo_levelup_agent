package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/api"
)

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("warning: failed to load ../.env: %v", err)
	}

	ctx := context.Background()
	server, err := api.NewServer(ctx)
	if err != nil {
		log.Fatalf("failed to initialize server: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	address := ":" + port

	httpServer := &http.Server{
		Addr:              address,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("Photo Levelup backend listening on %s", address)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
