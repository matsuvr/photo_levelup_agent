package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("Shutting down gracefully...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		if err := server.Close(); err != nil {
			log.Printf("Resource cleanup error: %v", err)
		}
	}()

	log.Printf("Photo Levelup backend listening on :%s", port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
