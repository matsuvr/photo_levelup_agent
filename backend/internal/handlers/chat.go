package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

type chatRequest struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

type chatResponse struct {
	Reply string `json:"reply"`
}

type ChatHandler struct {
	deps *Dependencies
}

func NewChatHandler(deps *Dependencies) *ChatHandler {
	return &ChatHandler{deps: deps}
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeJSONError(w, http.StatusBadRequest, "Message is required")
		return
	}
	if req.SessionID == "" {
		req.SessionID = "default"
	}

	ctx := r.Context()
	reply, err := chatWithAgent(ctx, h.deps, req.SessionID, req.Message)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, chatResponse{Reply: reply})
}

func chatWithAgent(ctx context.Context, deps *Dependencies, sessionID, message string) (string, error) {
	runner, err := runner.New(runner.Config{
		AppName:        "photo_levelup",
		Agent:          deps.Agent,
		SessionService: deps.SessionService,
	})
	if err != nil {
		return "", err
	}

	if _, err := deps.SessionService.Create(ctx, &session.CreateRequest{
		AppName:   "photo_levelup",
		UserID:    sessionID,
		SessionID: sessionID,
	}); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return "", err
		}
	}

	content := genai.NewContentFromText(message, genai.RoleUser)
	for event, err := range runner.Run(ctx, sessionID, sessionID, content, agent.RunConfig{}) {
		if err != nil {
			return "", err
		}
		if event == nil || !event.IsFinalResponse() {
			continue
		}
		text := strings.TrimSpace(extractText(event.Content))
		if text != "" {
			return text, nil
		}
	}

	return "", errors.New("chat response missing")
}
