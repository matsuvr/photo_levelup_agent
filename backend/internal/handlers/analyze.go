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
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"
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

	userID := r.FormValue("userId")
	if userID == "" {
		userID = "anonymous"
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

	log.Printf("INFO: Created job %s for session %s (user: %s)", jobID, sessionID, userID)

	// Build base URL for returning image proxy links
	baseURL := resolveBaseURL(r)

	// Start async processing
	go h.processAnalysis(jobID, userID, sessionID, imageData, contentType, baseURL)

	// Return job ID immediately
	writeJSON(w, http.StatusAccepted, map[string]string{
		"jobId":  jobID,
		"status": string(JobStatusPending),
	})
}

// processAnalysis runs the analysis in background
func (h *AnalyzeHandler) processAnalysis(jobID, userID, sessionID string, imageData []byte, contentType string, baseURL string) {
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
	analysis, err := analyzeWithAgent(ctx, h.deps, userID, sessionID, imageURL)
	if err != nil {
		log.Printf("ERROR: Job %s - Failed to analyze image: %v", jobID, err)
		jobStore.SetFailed(jobID, err.Error())
		return
	}

	// Generate enhanced image
	enhancedURL, err := generateEnhancedImage(ctx, storageClient, imageURL, analysis, baseURL)
	if err != nil {
		log.Printf("ERROR: Job %s - Failed to generate enhanced image: %v", jobID, err)
		jobStore.SetFailed(jobID, err.Error())
		return
	}

	// Update session state with all analysis data
	resolvedSessionID, resolveErr := resolveSessionID(ctx, h.deps.SessionService, "photo_levelup", userID, sessionID)
	if resolveErr != nil {
		log.Printf("ERROR: Job %s - Failed to resolve session ID for user %s, session %s: %v", jobID, userID, sessionID, resolveErr)
	}
	if resolvedSessionID != "" {
		// Serialize analysis result to JSON
		analysisJSON, err := json.Marshal(analysis)
		if err != nil {
			log.Printf("WARN: Job %s - Failed to marshal analysis: %v", jobID, err)
		}

		stateUpdates := map[string]any{
			"enhanced_image_url":  enhancedURL,
			"original_image_url":  imageURL,
			"frontend_session_id": sessionID,
			"overall_score":       analysis.OverallScore,
			"title":               analysis.PhotoSummary,
		}
		if analysisJSON != nil {
			stateUpdates["analysis_result"] = string(analysisJSON)
		}

		if err := updateSessionState(ctx, h.deps.SessionService, userID, resolvedSessionID, stateUpdates); err != nil {
			log.Printf("WARN: Job %s - Failed to update session state: %v", jobID, err)
		}

		// Seed conversation events so the ADK runner sees prior context
		if err := seedAnalysisEvents(ctx, h.deps.SessionService, userID, resolvedSessionID, analysis); err != nil {
			log.Printf("WARN: Job %s - Failed to seed analysis events: %v", jobID, err)
		}
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

func analyzeWithAgent(ctx context.Context, deps *Dependencies, userID, sessionID string, imageURL string) (*services.AnalysisResult, error) {
	log.Printf("INFO: Starting direct image analysis for user %s, session %s", userID, sessionID)

	// Call Gemini API directly for reliable image analysis
	geminiClient := services.NewGeminiClient()
	result, err := geminiClient.AnalyzeImage(ctx, imageURL)
	if err != nil {
		log.Printf("ERROR: Direct image analysis failed: %v", err)
		return nil, fmt.Errorf("画像分析に失敗しました: %w", err)
	}

	log.Printf("INFO: Direct image analysis completed successfully for user %s, session %s", userID, sessionID)
	return result, nil
}

func generateEnhancedImage(
	ctx context.Context,
	storageClient *services.StorageClient,
	imageURL string,
	analysis *services.AnalysisResult,
	baseURL string,
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

	return buildImageProxyURL(baseURL, objectName)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func resolveBaseURL(r *http.Request) string {
	if envURL := strings.TrimSpace(os.Getenv("PUBLIC_BACKEND_BASE_URL")); envURL != "" {
		return envURL
	}
	if envURL := strings.TrimSpace(os.Getenv("BACKEND_BASE_URL")); envURL != "" {
		return envURL
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s", proto, host)
}

func buildImageProxyURL(baseURL string, objectName string) (string, error) {
	if strings.TrimSpace(objectName) == "" {
		return "", errors.New("object name is required")
	}
	escaped := url.QueryEscape(objectName)
	if strings.TrimSpace(baseURL) == "" {
		return "", errors.New("base url is required")
	}
	return fmt.Sprintf("%s/photo/image?object=%s", strings.TrimRight(baseURL, "/"), escaped), nil
}

// StateUpdater is an optional interface for session services that support direct state updates.
type StateUpdater interface {
	UpdateState(ctx context.Context, appName, userID, sessionID string, updates map[string]any) error
}

// seedAnalysisEvents injects synthetic user+model events into the session so
// the ADK runner replays them as conversation history on follow-up chats.
func seedAnalysisEvents(ctx context.Context, sessionService session.Service, userID, resolvedSessionID string, analysis *services.AnalysisResult) error {
	sessResp, err := sessionService.Get(ctx, &session.GetRequest{
		AppName:   "photo_levelup",
		UserID:    userID,
		SessionID: resolvedSessionID,
	})
	if err != nil {
		return fmt.Errorf("failed to get session for seeding events: %w", err)
	}
	sess := sessResp.Session

	invocationID := uuid.New().String()

	// 1. User event: "analyze this photo" (text only; gs:// URIs are not
	//    accessible to the Gemini API via API key)
	userEvent := session.NewEvent(invocationID)
	userEvent.Author = "user"
	userEvent.Content = genai.NewContentFromText("この写真を分析して改善点を教えてください", genai.RoleUser)

	if err := sessionService.AppendEvent(ctx, sess, userEvent); err != nil {
		return fmt.Errorf("failed to append user event: %w", err)
	}

	// 2. Model event: analysis summary
	summary := fmt.Sprintf("写真を分析しました。\n\n**%s**\n総合スコア: %d/10\n\n%s",
		analysis.PhotoSummary, analysis.OverallScore, analysis.Summary)

	modelEvent := session.NewEvent(invocationID)
	modelEvent.Author = "photo_coach"
	modelEvent.Content = genai.NewContentFromText(summary, genai.RoleModel)

	if err := sessionService.AppendEvent(ctx, sess, modelEvent); err != nil {
		return fmt.Errorf("failed to append model event: %w", err)
	}

	log.Printf("INFO: Seeded analysis events for session %s", resolvedSessionID)
	return nil
}

func updateSessionState(ctx context.Context, sessionService session.Service, userID, sessionID string, updates map[string]any) error {
	// Try to use direct state update if the session service supports it
	if updater, ok := sessionService.(StateUpdater); ok {
		if err := updater.UpdateState(ctx, "photo_levelup", userID, sessionID, updates); err != nil {
			log.Printf("WARN: Direct state update failed, falling back to memory-only update: %v", err)
		} else {
			log.Printf("INFO: Session state updated and persisted for session %s", sessionID)
			return nil
		}
	}

	// Fallback: update state in memory (may not persist for all session service types)
	response, err := sessionService.Get(ctx, &session.GetRequest{
		AppName:   "photo_levelup",
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return err
	}

	state := response.Session.State()
	for key, value := range updates {
		if err := state.Set(key, value); err != nil {
			log.Printf("WARN: Failed to set state key %s: %v", key, err)
		}
	}

	log.Printf("WARN: Session state updated in memory only for session %s (may not persist)", sessionID)
	return nil
}
