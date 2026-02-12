package handlers

import (
	"github.com/matsuvr/photo_levelup_agent/backend/internal/middleware"
)

type SessionAccessError struct {
	Message string
}

func (e *SessionAccessError) Error() string {
	return e.Message
}

func resolveUserID(auth middleware.AuthResult, fallbackUserID string) string {
	if !auth.IsAnonymous && auth.UserID != "" {
		return auth.UserID
	}
	if fallbackUserID != "" {
		return "anonymous:" + fallbackUserID
	}
	return "anonymous"
}
