package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/services"
)

// ImageHandler streams images stored in GCS through the backend.
type ImageHandler struct{}

// NewImageHandler creates a new image handler.
func NewImageHandler() *ImageHandler {
	return &ImageHandler{}
}

// ServeHTTP handles GET requests for image proxying.
func (h *ImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	objectName := strings.TrimSpace(r.URL.Query().Get("object"))
	if objectName == "" {
		writeJSONError(w, http.StatusBadRequest, "object is required")
		return
	}
	if !isSafeObjectName(objectName) {
		writeJSONError(w, http.StatusBadRequest, "invalid object")
		return
	}

	ctx := r.Context()
	storageClient, err := services.NewStorageClient(ctx)
	if err != nil {
		log.Printf("ERROR: ImageHandler storage client error: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "storage client error")
		return
	}

	reader, contentType, size, err := storageClient.OpenObject(ctx, objectName)
	if err != nil {
		log.Printf("ERROR: ImageHandler failed to open object %s: %v", objectName, err)
		writeJSONError(w, http.StatusNotFound, "image not found")
		return
	}
	defer reader.Close()

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}

	// Support download mode via ?download=true
	if r.URL.Query().Get("download") == "true" {
		prefix := "photo"
		if strings.HasPrefix(objectName, "uploads/") {
			prefix = "original"
		} else if strings.HasPrefix(objectName, "enhanced/") {
			prefix = "annotated"
		} else if strings.HasPrefix(objectName, "clean_enhanced/") {
			prefix = "enhanced"
		}
		// Extract filename from object path
		parts := strings.Split(objectName, "/")
		filename := parts[len(parts)-1]
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s_%s"`, prefix, filename))
	}

	if _, err := io.Copy(w, reader); err != nil {
		log.Printf("ERROR: ImageHandler failed to stream object %s: %v", objectName, err)
	}
}

func isSafeObjectName(objectName string) bool {
	if strings.HasPrefix(objectName, "/") {
		return false
	}
	if strings.Contains(objectName, "..") || strings.Contains(objectName, "\\") {
		return false
	}
	return strings.HasPrefix(objectName, "enhanced/") ||
		strings.HasPrefix(objectName, "clean_enhanced/") ||
		strings.HasPrefix(objectName, "uploads/")
}
