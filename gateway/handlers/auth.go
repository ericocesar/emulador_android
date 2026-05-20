package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/emulador/gateway/auth"
	"github.com/emulador/gateway/db"
)

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}

var registrationDisabled = true

func RegisterHandler(database *sql.DB, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if registrationDisabled {
			writeError(w, "registration is temporarily disabled", http.StatusForbidden)
			return
		}

		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		req.Name = strings.TrimSpace(req.Name)

		if req.Email == "" || req.Password == "" || req.Name == "" {
			writeError(w, "email, password, and name are required", http.StatusBadRequest)
			return
		}

		if len(req.Password) < 6 {
			writeError(w, "password must be at least 6 characters", http.StatusBadRequest)
			return
		}

		// Check if user already exists
		existing, _ := db.GetUserByEmail(database, req.Email)
		if existing != nil {
			writeError(w, "email already registered", http.StatusConflict)
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			log.Printf("error hashing password: %v", err)
			writeError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		user, err := db.CreateUser(database, req.Email, hash, req.Name)
		if err != nil {
			log.Printf("error creating user: %v", err)
			writeError(w, "failed to create user", http.StatusInternalServerError)
			return
		}

		token, err := auth.GenerateToken(user.ID, jwtSecret)
		if err != nil {
			log.Printf("error generating token: %v", err)
			writeError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"data": authResponse{Token: token, User: user},
		})
	}
}

func LoginHandler(database *sql.DB, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		if req.Email == "" || req.Password == "" {
			writeError(w, "email and password are required", http.StatusBadRequest)
			return
		}

		user, err := db.GetUserByEmail(database, req.Email)
		if err != nil {
			writeError(w, "invalid email or password", http.StatusUnauthorized)
			return
		}

		if !auth.CheckPassword(user.Password, req.Password) {
			writeError(w, "invalid email or password", http.StatusUnauthorized)
			return
		}

		token, err := auth.GenerateToken(user.ID, jwtSecret)
		if err != nil {
			log.Printf("error generating token: %v", err)
			writeError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": authResponse{Token: token, User: user},
		})
	}
}

func MeHandler(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := auth.GetUserID(r.Context())
		if userID == "" {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := db.GetUserByID(database, userID)
		if err != nil {
			writeError(w, "user not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": user,
		})
	}
}
