package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

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

func (s *StorageClient) OpenObject(ctx context.Context, objectName string) (*storage.Reader, string, int64, error) {
	if s.bucketName == "" {
		return nil, "", 0, fmt.Errorf("BUCKET_NAME is required")
	}
	if strings.TrimSpace(objectName) == "" {
		return nil, "", 0, fmt.Errorf("object name is required")
	}

	reader, err := s.client.Bucket(s.bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, "", 0, err
	}

	contentType := reader.Attrs.ContentType
	size := reader.Attrs.Size
	return reader, contentType, size, nil
}
