package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/services"
)

type analyzeResponse struct {
	EnhancedImageURL string                  `json:"enhancedImageUrl"`
	Analysis         services.AnalysisResult `json:"analysis"`
	InitialAdvice    string                  `json:"initialAdvice"`
}

type AnalyzeHandler struct {
	deps *Dependencies
}

func NewAnalyzeHandler(deps *Dependencies) *AnalyzeHandler {
	return &AnalyzeHandler{deps: deps}
}

func (h *AnalyzeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeJSONError(w, http.StatusBadRequest, "File too large")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Failed to get file")
		return
	}
	defer file.Close()

	sessionID := r.FormValue("sessionId")
	if sessionID == "" {
		sessionID = "default"
	}

	ctx := r.Context()
	storageClient, err := services.NewStorageClient(ctx)
	if err != nil {
		log.Printf("ERROR: Storage client error: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "Storage client error")
		return
	}

	processor := services.NewImageProcessor()
	resized, contentType, err := processor.ResizeToMaxEdge(file, header.Header.Get("Content-Type"))
	if err != nil {
		log.Printf("ERROR: Failed to resize image: %v", err)
		writeJSONError(w, http.StatusBadRequest, "Invalid image")
		return
	}
	log.Printf("INFO: Image resized successfully, size=%d bytes, contentType=%s", len(resized), contentType)

	imageURL, err := storageClient.UploadImage(ctx, resized, contentType)
	if err != nil {
		log.Printf("ERROR: Failed to upload image to GCS: %v", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	analysis, err := analyzeWithAgent(ctx, h.deps, sessionID, imageURL)
	if err != nil {
		log.Printf("ERROR: Failed to analyze image: %v", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	enhancedURL, err := generateEnhancedImage(ctx, storageClient, imageURL, analysis)
	if err != nil {
		log.Printf("ERROR: Failed to generate enhanced image: %v", err)
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := analyzeResponse{
		EnhancedImageURL: enhancedURL,
		Analysis:         *analysis,
		InitialAdvice:    analysis.Summary,
	}

	writeJSON(w, http.StatusOK, response)
}

func analyzeWithAgent(ctx context.Context, deps *Dependencies, sessionID string, imageURL string) (*services.AnalysisResult, error) {
	runner, err := runner.New(runner.Config{
		AppName:        "photo_levelup",
		Agent:          deps.Agent,
		SessionService: deps.SessionService,
	})
	if err != nil {
		return nil, err
	}

	if _, err := deps.SessionService.Create(ctx, &session.CreateRequest{
		AppName:   "photo_levelup",
		UserID:    sessionID,
		SessionID: sessionID,
	}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return nil, err
		}
	}

	content := genai.NewContentFromText(fmt.Sprintf("次の写真を分析してください。URL: %s", imageURL), genai.RoleUser)
	for event, err := range runner.Run(ctx, sessionID, sessionID, content, agent.RunConfig{}) {
		if err != nil {
			return nil, err
		}
		if event == nil || !event.IsFinalResponse() {
			continue
		}
		analysis, err := extractAnalysisFromState(ctx, deps.SessionService, sessionID)
		if err != nil {
			return nil, err
		}
		return analysis, nil
	}

	return nil, errors.New("analysis result missing")
}

func generateEnhancedImage(
	ctx context.Context,
	storageClient *services.StorageClient,
	imageURL string,
	analysis *services.AnalysisResult,
) (string, error) {
	geminiClient := services.NewGeminiClient()
	result, err := geminiClient.EnhancePhoto(ctx, services.EnhancementInput{
		ImageURL: imageURL,
		Analysis: analysis,
	})
	if err != nil {
		return "", err
	}

	imageData, err := base64.StdEncoding.DecodeString(result.ImageBase64)
	if err != nil {
		return "", err
	}

	_, objectName, err := storageClient.UploadImageWithPrefix(ctx, imageData, "image/jpeg", "enhanced")
	if err != nil {
		return "", err
	}

	return storageClient.SignedURL(ctx, objectName)
}

func extractAnalysisFromState(
	ctx context.Context,
	sessionService session.Service,
	sessionID string,
) (*services.AnalysisResult, error) {
	response, err := sessionService.Get(ctx, &session.GetRequest{
		AppName:   "photo_levelup",
		UserID:    sessionID,
		SessionID: sessionID,
	})
	if err != nil {
		return nil, err
	}

	stored, err := response.Session.State().Get("analysis_result")
	if err != nil {
		return nil, err
	}
	value, ok := stored.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return nil, errors.New("analysis_result is empty")
	}

	var analysis services.AnalysisResult
	if err := json.Unmarshal([]byte(value), &analysis); err != nil {
		return nil, fmt.Errorf("analysis_result parse failed: %w", err)
	}
	return &analysis, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
