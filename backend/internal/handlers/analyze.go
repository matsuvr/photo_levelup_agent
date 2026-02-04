package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/services"
)

// AnalyzeHandler handles photo analysis requests
type AnalyzeHandler struct {
	deps *Dependencies
}

// NewAnalyzeHandler creates a new analyze handler
func NewAnalyzeHandler(deps *Dependencies) *AnalyzeHandler {
	return &AnalyzeHandler{deps: deps}
}

// ServeHTTP handles POST requests to start an async analysis job
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

	// Read file into memory for async processing
	imageData, err := io.ReadAll(file)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Failed to read file")
		return
	}
	contentType := header.Header.Get("Content-Type")

	// Create job and return immediately
	jobID := uuid.New().String()
	jobStore := GetJobStore()
	jobStore.Create(jobID)

	log.Printf("INFO: Created job %s for session %s", jobID, sessionID)

	// Start async processing
	go h.processAnalysis(jobID, sessionID, imageData, contentType)

	// Return job ID immediately
	writeJSON(w, http.StatusAccepted, map[string]string{
		"jobId":  jobID,
		"status": string(JobStatusPending),
	})
}

// processAnalysis runs the analysis in background
func (h *AnalyzeHandler) processAnalysis(jobID, sessionID string, imageData []byte, contentType string) {
	jobStore := GetJobStore()
	jobStore.SetProcessing(jobID)

	ctx := context.Background()

	// Storage client
	storageClient, err := services.NewStorageClient(ctx)
	if err != nil {
		log.Printf("ERROR: Job %s - Storage client error: %v", jobID, err)
		jobStore.SetFailed(jobID, "Storage client error")
		return
	}

	// Resize image
	processor := services.NewImageProcessor()
	resized, resizedContentType, err := processor.ResizeToMaxEdgeFromBytes(imageData, contentType)
	if err != nil {
		log.Printf("ERROR: Job %s - Failed to resize image: %v", jobID, err)
		jobStore.SetFailed(jobID, "Invalid image")
		return
	}
	log.Printf("INFO: Job %s - Image resized successfully, size=%d bytes", jobID, len(resized))

	// Upload to GCS
	imageURL, err := storageClient.UploadImage(ctx, resized, resizedContentType)
	if err != nil {
		log.Printf("ERROR: Job %s - Failed to upload image to GCS: %v", jobID, err)
		jobStore.SetFailed(jobID, err.Error())
		return
	}

	// Analyze with agent
	analysis, err := analyzeWithAgent(ctx, h.deps, sessionID, imageURL)
	if err != nil {
		log.Printf("ERROR: Job %s - Failed to analyze image: %v", jobID, err)
		jobStore.SetFailed(jobID, err.Error())
		return
	}

	// Generate enhanced image
	enhancedURL, err := generateEnhancedImage(ctx, storageClient, imageURL, analysis)
	if err != nil {
		log.Printf("ERROR: Job %s - Failed to generate enhanced image: %v", jobID, err)
		jobStore.SetFailed(jobID, err.Error())
		return
	}

	// Mark as completed
	result := &AnalyzeResult{
		EnhancedImageURL: enhancedURL,
		Analysis:         *analysis,
		InitialAdvice:    analysis.Summary,
	}
	jobStore.SetCompleted(jobID, result)
	log.Printf("INFO: Job %s - Completed successfully", jobID)
}

// AnalyzeStatusHandler handles job status queries
type AnalyzeStatusHandler struct{}

// NewAnalyzeStatusHandler creates a new status handler
func NewAnalyzeStatusHandler() *AnalyzeStatusHandler {
	return &AnalyzeStatusHandler{}
}

// ServeHTTP handles GET requests to check job status
func (h *AnalyzeStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	jobID := r.URL.Query().Get("jobId")
	if jobID == "" {
		writeJSONError(w, http.StatusBadRequest, "jobId is required")
		return
	}

	jobStore := GetJobStore()
	job, ok := jobStore.Get(jobID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Job not found")
		return
	}

	response := map[string]interface{}{
		"jobId":  job.ID,
		"status": string(job.Status),
	}

	if job.Status == JobStatusCompleted && job.Result != nil {
		response["result"] = job.Result
	}

	if job.Status == JobStatusFailed {
		response["error"] = job.Error
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

	resolvedSessionID, err := resolveSessionID(ctx, deps.SessionService, "photo_levelup", sessionID)
	if err != nil {
		return nil, err
	}

	content := genai.NewContentFromText(fmt.Sprintf("analyze_photo ツールを使って次の写真を分析してください。画像URL: %s", imageURL), genai.RoleUser)
	log.Printf("INFO: Starting agent run for session %s (resolved: %s)", sessionID, resolvedSessionID)
	for event, err := range runner.Run(ctx, sessionID, resolvedSessionID, content, agent.RunConfig{}) {
		if err != nil {
			log.Printf("ERROR: Agent run error: %v", err)
			return nil, err
		}

		log.Printf("DEBUG: Agent event received. IsFinal: %v", event.IsFinalResponse())
		if event.Content != nil {
			log.Printf("DEBUG: Agent content: %s", extractText(event.Content))
		}

		if event == nil || !event.IsFinalResponse() {
			continue
		}
		analysis, err := extractAnalysisFromState(ctx, deps.SessionService, sessionID, resolvedSessionID)
		if err != nil {
			log.Printf("ERROR: Failed to extract analysis from state: %v", err)
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
	resolvedSessionID string,
) (*services.AnalysisResult, error) {
	response, err := sessionService.Get(ctx, &session.GetRequest{
		AppName:   "photo_levelup",
		UserID:    sessionID,
		SessionID: resolvedSessionID,
	})
	if err != nil {
		return nil, err
	}

	stored, err := response.Session.State().Get("analysis_result")
	if err != nil {
		// Log available keys for debugging
		log.Printf("ERROR: analysis_result key not found in state. Error: %v", err)
		return nil, err
	}
	value, ok := stored.(string)
	if !ok || strings.TrimSpace(value) == "" {
		log.Printf("ERROR: analysis_result is empty or invalid type: %T", stored)
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
