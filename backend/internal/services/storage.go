package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

type StorageClient struct {
	client     *storage.Client
	bucketName string
}

func NewStorageClient(ctx context.Context) (*StorageClient, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &StorageClient{
		client:     client,
		bucketName: os.Getenv("BUCKET_NAME"),
	}, nil
}

func (s *StorageClient) UploadImage(ctx context.Context, data []byte, contentType string) (string, error) {
	if s.bucketName == "" {
		return "", fmt.Errorf("BUCKET_NAME is required")
	}

	objectName := fmt.Sprintf("uploads/%s", uuid.NewString())
	obj := s.client.Bucket(s.bucketName).Object(objectName)
	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType

	if _, err := writer.Write(data); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	return fmt.Sprintf("gs://%s/%s", s.bucketName, objectName), nil
}

func (s *StorageClient) UploadImageWithPrefix(ctx context.Context, data []byte, contentType, prefix string) (string, string, error) {
	if s.bucketName == "" {
		return "", "", fmt.Errorf("BUCKET_NAME is required")
	}

	trimmedPrefix := strings.Trim(prefix, "/")
	if trimmedPrefix == "" {
		trimmedPrefix = "uploads"
	}

	objectName := fmt.Sprintf("%s/%s", trimmedPrefix, uuid.NewString())
	obj := s.client.Bucket(s.bucketName).Object(objectName)
	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType

	if _, err := writer.Write(data); err != nil {
		return "", "", err
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}

	return fmt.Sprintf("gs://%s/%s", s.bucketName, objectName), objectName, nil
}

func (s *StorageClient) SignedURL(ctx context.Context, objectName string) (string, error) {
	if s.bucketName == "" {
		return "", fmt.Errorf("BUCKET_NAME is required")
	}
	if strings.TrimSpace(objectName) == "" {
		return "", fmt.Errorf("object name is required")
	}

	opts := &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(24 * time.Hour),
	}

	return s.client.Bucket(s.bucketName).SignedURL(objectName, opts)
}

func (s *StorageClient) SignedURLFromGCSURL(ctx context.Context, gcsURL string) (string, error) {
	if s.bucketName == "" {
		return "", fmt.Errorf("BUCKET_NAME is required")
	}

	trimmed := strings.TrimPrefix(gcsURL, "gs://")
	if trimmed == gcsURL {
		return "", fmt.Errorf("invalid gcs url")
	}

	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid gcs url")
	}
	if parts[0] != s.bucketName {
		return "", fmt.Errorf("bucket mismatch")
	}

	return s.SignedURL(ctx, parts[1])
}

func (s *StorageClient) UploadFromReader(ctx context.Context, reader io.Reader, contentType string) (string, error) {
	if s.bucketName == "" {
		return "", fmt.Errorf("BUCKET_NAME is required")
	}

	objectName := fmt.Sprintf("uploads/%s", uuid.NewString())
	obj := s.client.Bucket(s.bucketName).Object(objectName)
	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType

	if _, err := io.Copy(writer, reader); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	return fmt.Sprintf("gs://%s/%s", s.bucketName, objectName), nil
}
