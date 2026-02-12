package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/auth"
)

type contextKey string

const (
	UserIDKey      contextKey = "userId"
	IsAnonymousKey contextKey = "isAnonymous"
)

type AuthResult struct {
	UserID      string
	IsAnonymous bool
}

func AuthMiddleware(firebaseAuth *auth.FirebaseAuthClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if !firebaseAuth.IsEnabled() {
				log.Println("DEBUG: Authentication disabled (dev mode)")
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				ctx = context.WithValue(ctx, IsAnonymousKey, true)
				ctx = context.WithValue(ctx, UserIDKey, "")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeAuthError(w, "Invalid authorization format")
				return
			}

			idToken := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := firebaseAuth.VerifyIDToken(ctx, idToken)
			if err != nil {
				log.Printf("WARN: Token verification failed: %v", err)
				writeAuthError(w, "Invalid token")
				return
			}

			ctx = context.WithValue(ctx, IsAnonymousKey, false)
			ctx = context.WithValue(ctx, UserIDKey, token.UID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func GetAuthResult(ctx context.Context) AuthResult {
	result := AuthResult{IsAnonymous: true}

	if isAnon, ok := ctx.Value(IsAnonymousKey).(bool); ok {
		result.IsAnonymous = isAnon
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		result.UserID = userID
	}

	return result
}
