package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/emulador/gateway/auth"
	"github.com/emulador/gateway/db"
)

type createTokenRequest struct {
	Name string `json:"name"`
}

type createTokenResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"` // only returned once, on creation
}

// generateAPIToken returns a (plaintext, prefix) pair.
// Format: astra_<32 hex chars>. Prefix = first 12 chars of plaintext (for display).
func generateAPIToken() (plaintext, prefix string, err error) {
	buf := make([]byte, 16)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	plaintext = "astra_" + hex.EncodeToString(buf)
	prefix = plaintext[:12]
	return plaintext, prefix, nil
}

// POST /api/tokens
func CreateAPITokenHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body createTokenRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		name := strings.TrimSpace(body.Name)
		if name == "" {
			name = "default"
		}
		if len(name) > 120 {
			name = name[:120]
		}

		plaintext, prefix, err := generateAPIToken()
		if err != nil {
			writeError(w, "failed to generate token: "+err.Error(), http.StatusInternalServerError)
			return
		}
		hash := auth.HashAPIToken(plaintext)
		tok, err := db.CreateAPIToken(database, userID, name, prefix, hash)
		if err != nil {
			writeError(w, "failed to save token: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, createTokenResponse{
			ID:    tok.ID,
			Name:  tok.Name,
			Token: plaintext, // user must copy NOW — never returned again
		})
	}
}

// GET /api/tokens
func ListAPITokensHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ts, err := db.ListAPITokensByUser(database, userID)
		if err != nil {
			writeError(w, "failed to list tokens: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": ts})
	}
}

// DELETE /api/tokens/:id
func RevokeAPITokenHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := mux.Vars(r)["id"]
		n, err := db.RevokeAPIToken(database, id, userID)
		if err != nil {
			writeError(w, "failed to revoke: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if n == 0 {
			writeError(w, "token not found or already revoked", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}
