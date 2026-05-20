package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/emulador/gateway/db"
)

type contextKey string

const UserIDKey contextKey = "userID"

// HashAPIToken returns the canonical sha256 hex digest used to look up a token
// in the database.
func HashAPIToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// AuthMiddleware accepts either a JWT Bearer token (panel sessions) or
// a personal API token starting with "astra_" (programmatic CRUD calls).
func AuthMiddleware(secret string, database *sql.DB) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				http.Error(w, `{"error":"invalid authorization header format"}`, http.StatusUnauthorized)
				return
			}
			cred := parts[1]

			// API token path
			if strings.HasPrefix(cred, "astra_") && database != nil {
				tok, err := db.GetAPITokenByHash(database, HashAPIToken(cred))
				if err != nil {
					http.Error(w, `{"error":"invalid or revoked api token"}`, http.StatusUnauthorized)
					return
				}
				go db.TouchAPITokenLastUsed(database, tok.ID)
				ctx := context.WithValue(r.Context(), UserIDKey, tok.UserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// JWT path
			userID, err := ValidateToken(cred, secret)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}
