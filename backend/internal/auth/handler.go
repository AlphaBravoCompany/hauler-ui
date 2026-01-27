package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Handler handles HTTP requests for authentication operations
type Handler struct {
	manager *Manager
}

// NewHandler creates a new auth handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// LoginRequest represents the login request
type LoginRequest struct {
	Password string `json:"password"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

// Login handles login requests
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If auth is not enabled, return success without checking password
	if !h.manager.IsEnabled() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(LoginResponse{
			Success: true,
			Message: "Authentication is not configured",
		})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if !h.manager.VerifyPassword(req.Password) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Invalid password",
		})
		return
	}

	token, expiresAt, err := h.manager.CreateSession()
	if err != nil {
		log.Printf("Error creating session: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Failed to create session",
		})
		return
	}

	SetSessionCookie(w, token, expiresAt)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(LoginResponse{
		Success:   true,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}

// Logout handles logout requests
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractTokenFromRequest(r)
	if token != "" {
		if err := h.manager.DeleteSession(token); err != nil {
			log.Printf("Error deleting session: %v", err)
		}
	}

	ClearSessionCookie(w)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}

// Validate handles validation requests to check if a session is valid
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractTokenFromRequest(r)
	valid := h.manager.ValidateSession(token)
	enabled := h.manager.IsEnabled()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": valid,
		"authEnabled":   enabled,
	})
}

// RegisterRoutes registers the auth routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}
		h.Login(w, r)
	})

	mux.HandleFunc("/api/auth/logout", h.Logout)
	mux.HandleFunc("/api/auth/validate", h.Validate)
}
