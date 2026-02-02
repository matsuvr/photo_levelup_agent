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
	ImageURL  string `json:"imageUrl,omitempty"`
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
	reply, err := chatWithAgent(ctx, h.deps, req.SessionID, req.Message, req.ImageURL)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, chatResponse{Reply: reply})
}

func chatWithAgent(ctx context.Context, deps *Dependencies, sessionID, message, imageURL string) (string, error) {
	runner, err := runner.New(runner.Config{
		AppName:        "photo_levelup",
		Agent:          deps.Agent,
		SessionService: deps.SessionService,
	})
	if err != nil {
		return "", err
	}

	resolvedSessionID, err := resolveSessionID(ctx, deps.SessionService, "photo_levelup", sessionID)
	if err != nil {
		return "", err
	}

	var content *genai.Content
	if imageURL != "" {
		// Include image with message
		parts := []*genai.Part{
			genai.NewPartFromText(message),
			genai.NewPartFromURI(imageURL, "image/jpeg"),
		}
		content = genai.NewContentFromParts(parts, genai.RoleUser)
	} else {
		content = genai.NewContentFromText(message, genai.RoleUser)
	}
	for event, err := range runner.Run(ctx, sessionID, resolvedSessionID, content, agent.RunConfig{}) {
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

func resolveSessionID(ctx context.Context, sessionService session.Service, appName, userID string) (string, error) {
	listResponse, err := sessionService.List(ctx, &session.ListRequest{
		AppName: appName,
		UserID:  userID,
	})
	if err != nil {
		if createResponse, createErr := sessionService.Create(ctx, &session.CreateRequest{
			AppName: appName,
			UserID:  userID,
		}); createErr == nil {
			newSessionID := createResponse.Session.ID()
			if strings.TrimSpace(newSessionID) != "" {
				return newSessionID, nil
			}
		}
		return "", err
	}

	if len(listResponse.Sessions) > 0 {
		latest := listResponse.Sessions[0]
		latestTime := latest.LastUpdateTime()
		for _, candidate := range listResponse.Sessions[1:] {
			if candidate.LastUpdateTime().After(latestTime) {
				latest = candidate
				latestTime = candidate.LastUpdateTime()
			}
		}
		return latest.ID(), nil
	}

	createResponse, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: appName,
		UserID:  userID,
	})
	if err != nil {
		return "", err
	}

	newSessionID := createResponse.Session.ID()
	if strings.TrimSpace(newSessionID) == "" {
		return "", errors.New("created session has empty ID")
	}

	return newSessionID, nil
}
