package auth

import (
	"context"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

type FirebaseAuthClient struct {
	client  *auth.Client
	enabled bool
}

func NewFirebaseAuthClient(ctx context.Context) (*FirebaseAuthClient, error) {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		log.Println("INFO: GOOGLE_CLOUD_PROJECT not set, authentication disabled")
		return &FirebaseAuthClient{enabled: false}, nil
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: projectID,
	})
	if err != nil {
		return nil, err
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("INFO: Firebase Auth client initialized for project: %s", projectID)
	return &FirebaseAuthClient{client: client, enabled: true}, nil
}

func (c *FirebaseAuthClient) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	if !c.enabled {
		return nil, nil
	}
	return c.client.VerifyIDToken(ctx, idToken)
}

func (c *FirebaseAuthClient) IsEnabled() bool {
	return c.enabled
}
