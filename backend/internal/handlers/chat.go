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
	UserID    string `json:"userId"`
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
	if req.UserID == "" {
		req.UserID = "anonymous"
	}

	ctx := r.Context()
	reply, err := chatWithAgent(ctx, h.deps, req.UserID, req.SessionID, req.Message, req.ImageURL)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, chatResponse{Reply: reply})
}

func chatWithAgent(ctx context.Context, deps *Dependencies, userID, sessionID, message, imageURL string) (string, error) {
	runner, err := runner.New(runner.Config{
		AppName:        "photo_levelup",
		Agent:          deps.Agent,
		SessionService: deps.SessionService,
	})
	if err != nil {
		return "", err
	}

	resolvedSessionID, err := resolveSessionID(ctx, deps.SessionService, "photo_levelup", userID, sessionID)
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
	for event, err := range runner.Run(ctx, userID, resolvedSessionID, content, agent.RunConfig{}) {
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

// resolveSessionID finds or creates an ADK session for the given user and frontend sessionId.
// The sessionId parameter is used to map frontend sessions to ADK sessions.
// When a specific sessionId is provided (not "default"), we look for an existing ADK session
// or create a new one associated with that sessionId.
func resolveSessionID(ctx context.Context, sessionService session.Service, appName, userID, sessionID string) (string, error) {
	// List all sessions for this user
	listResponse, err := sessionService.List(ctx, &session.ListRequest{
		AppName: appName,
		UserID:  userID,
	})
	if err != nil {
		// If listing fails, try to create a new session
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

	// If a specific sessionId was provided, look for a matching session
	// by checking if any session's state has this sessionId stored
	if sessionID != "default" && sessionID != "" {
		for _, sess := range listResponse.Sessions {
			if storedSessionID, err := sess.State().Get("frontend_session_id"); err == nil {
				if s, ok := storedSessionID.(string); ok && s == sessionID {
					return sess.ID(), nil
				}
			}
		}
		// No matching session found for the given sessionID - create a new one
		// This ensures each new frontend session gets its own backend session
		// Include frontend_session_id in initial state for session mapping
		createResponse, err := sessionService.Create(ctx, &session.CreateRequest{
			AppName: appName,
			UserID:  userID,
			State: map[string]any{
				"frontend_session_id": sessionID,
			},
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

	// For "default" or empty sessionId, use the most recent session or create new
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

	// Create a new session
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
