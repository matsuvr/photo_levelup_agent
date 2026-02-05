package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"google.golang.org/adk/session"
)

// SessionInfo represents a session summary for the list API
type SessionInfo struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	OverallScore *float64  `json:"overallScore,omitempty"`
	PhotoURL     string    `json:"photoUrl,omitempty"`
	MessageCount int       `json:"messageCount"`
}

// SessionDetail represents a full session with conversation history
type SessionDetail struct {
	SessionInfo
	Messages       []MessageInfo   `json:"messages"`
	AnalysisResult json.RawMessage `json:"analysisResult,omitempty"`
	OriginalImage  string          `json:"originalImageUrl,omitempty"`
}

// MessageInfo represents a chat message
type MessageInfo struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// SessionsHandler handles session list requests
type SessionsHandler struct {
	deps *Dependencies
}

// NewSessionsHandler creates a new sessions handler
func NewSessionsHandler(deps *Dependencies) *SessionsHandler {
	return &SessionsHandler{deps: deps}
}

// ServeHTTP handles GET /photo/sessions
func (h *SessionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		writeJSONError(w, http.StatusBadRequest, "userId is required")
		return
	}

	ctx := r.Context()
	sessions, err := h.listUserSessions(ctx, userID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list sessions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
	})
}

func (h *SessionsHandler) listUserSessions(ctx context.Context, userID string) ([]SessionInfo, error) {
	listResponse, err := h.deps.SessionService.List(ctx, &session.ListRequest{
		AppName: "photo_levelup",
		UserID:  userID,
	})
	if err != nil {
		return []SessionInfo{}, nil // Return empty list if no sessions
	}

	sessions := make([]SessionInfo, 0, len(listResponse.Sessions))
	for _, sess := range listResponse.Sessions {
		info := SessionInfo{
			ID:        sess.ID(),
			UserID:    userID,
			UpdatedAt: sess.LastUpdateTime(),
		}

		// Extract metadata from state
		state := sess.State()
		if title, err := state.Get("title"); err == nil {
			if s, ok := title.(string); ok {
				info.Title = s
			}
		}
		if info.Title == "" {
			info.Title = formatSessionTitle(sess.LastUpdateTime())
		}

		if createdAt, err := state.Get("created_at"); err == nil {
			if s, ok := createdAt.(string); ok {
				if t, parseErr := time.Parse(time.RFC3339, s); parseErr == nil {
					info.CreatedAt = t
				}
			}
		}
		if info.CreatedAt.IsZero() {
			info.CreatedAt = sess.LastUpdateTime()
		}

		if score, err := state.Get("overall_score"); err == nil {
			switch v := score.(type) {
			case float64:
				info.OverallScore = &v
			case int:
				f := float64(v)
				info.OverallScore = &f
			}
		}

		if photoURL, err := state.Get("enhanced_image_url"); err == nil {
			if s, ok := photoURL.(string); ok {
				info.PhotoURL = s
			}
		}

		// Count messages from events
		info.MessageCount = sess.Events().Len()

		sessions = append(sessions, info)
	}

	return sessions, nil
}

// SessionDetailHandler handles session detail requests
type SessionDetailHandler struct {
	deps *Dependencies
}

// NewSessionDetailHandler creates a new session detail handler
func NewSessionDetailHandler(deps *Dependencies) *SessionDetailHandler {
	return &SessionDetailHandler{deps: deps}
}

// ServeHTTP handles GET /photo/sessions/{sessionId}
func (h *SessionDetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract sessionId from path
	path := r.URL.Path
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) < 3 {
		writeJSONError(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	sessionID := parts[len(parts)-1]

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		writeJSONError(w, http.StatusBadRequest, "userId is required")
		return
	}

	ctx := r.Context()
	detail, err := h.getSessionDetail(ctx, userID, sessionID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Session not found")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *SessionDetailHandler) getSessionDetail(ctx context.Context, userID, sessionID string) (*SessionDetail, error) {
	getResponse, err := h.deps.SessionService.Get(ctx, &session.GetRequest{
		AppName:   "photo_levelup",
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return nil, err
	}

	sess := getResponse.Session
	state := sess.State()

	detail := &SessionDetail{
		SessionInfo: SessionInfo{
			ID:        sess.ID(),
			UserID:    userID,
			UpdatedAt: sess.LastUpdateTime(),
		},
		Messages: make([]MessageInfo, 0),
	}

	// Extract metadata from state
	if title, err := state.Get("title"); err == nil {
		if s, ok := title.(string); ok {
			detail.Title = s
		}
	}
	if detail.Title == "" {
		detail.Title = formatSessionTitle(sess.LastUpdateTime())
	}

	if createdAt, err := state.Get("created_at"); err == nil {
		if s, ok := createdAt.(string); ok {
			if t, parseErr := time.Parse(time.RFC3339, s); parseErr == nil {
				detail.CreatedAt = t
			}
		}
	}
	if detail.CreatedAt.IsZero() {
		detail.CreatedAt = sess.LastUpdateTime()
	}

	if score, err := state.Get("overall_score"); err == nil {
		switch v := score.(type) {
		case float64:
			detail.OverallScore = &v
		case int:
			f := float64(v)
			detail.OverallScore = &f
		}
	}

	if photoURL, err := state.Get("enhanced_image_url"); err == nil {
		if s, ok := photoURL.(string); ok {
			detail.PhotoURL = s
		}
	}

	if originalURL, err := state.Get("original_image_url"); err == nil {
		if s, ok := originalURL.(string); ok {
			detail.OriginalImage = s
		}
	}

	if analysisResult, err := state.Get("analysis_result"); err == nil {
		if s, ok := analysisResult.(string); ok {
			detail.AnalysisResult = json.RawMessage(s)
		}
	}

	// Extract messages from events
	events := sess.Events()
	for i := 0; i < events.Len(); i++ {
		event := events.At(i)
		if event.Content == nil {
			continue
		}

		var role string
		switch event.Content.Role {
		case "user":
			role = "user"
		case "model":
			role = "agent"
		default:
			continue
		}

		var content string
		if event.Content.Parts != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					content += part.Text
				}
			}
		}

		if content != "" {
			msg := MessageInfo{
				Role:      role,
				Content:   content,
				Timestamp: event.Timestamp,
			}
			detail.Messages = append(detail.Messages, msg)
		}
	}

	detail.MessageCount = len(detail.Messages)

	return detail, nil
}

func formatSessionTitle(t time.Time) string {
	loc, _ := time.LoadLocation("Asia/Tokyo")
	if loc != nil {
		t = t.In(loc)
	}
	return t.Format("1月2日 15:04")
}
