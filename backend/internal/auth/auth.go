package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
)

const (
	sessionCookieName = "haulerci_session"
	sessionDuration   = 24 * time.Hour
	tokenLength       = 32
)

// Manager handles authentication operations
type Manager struct {
	db        *sql.DB
	password  string
	enabled   bool
}

// NewManager creates a new auth manager
func NewManager(db *sql.DB, cfg *config.Config) *Manager {
	return &Manager{
		db:       db,
		password: cfg.UIPassword,
		enabled:  cfg.UIPassword != "",
	}
}

// IsEnabled returns whether authentication is enabled
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// VerifyPassword checks if the provided password matches the configured password
func (m *Manager) VerifyPassword(password string) bool {
	if !m.enabled {
		return true // No auth configured, allow access
	}
	// Hash the provided password and compare with stored hash
	hashed := hashPassword(password)
	return hashed == hashPassword(m.password)
}

// CreateSession creates a new session token and returns it
func (m *Manager) CreateSession() (string, time.Time, error) {
	token, err := generateToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generating token: %w", err)
	}

	expiresAt := time.Now().Add(sessionDuration)

	_, err = m.db.Exec(
		`INSERT INTO sessions (token, expires_at) VALUES (?, ?)`,
		token, expiresAt,
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("inserting session: %w", err)
	}

	// Clean up expired sessions
	go m.cleanupExpiredSessions()

	return token, expiresAt, nil
}

// ValidateSession checks if a session token is valid
func (m *Manager) ValidateSession(token string) bool {
	if !m.enabled {
		return true
	}

	var expiresAt time.Time
	err := m.db.QueryRow(
		`SELECT expires_at FROM sessions WHERE token = ? AND expires_at > CURRENT_TIMESTAMP`,
		token,
	).Scan(&expiresAt)

	return err == nil
}

// DeleteSession removes a session token
func (m *Manager) DeleteSession(token string) error {
	_, err := m.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// cleanupExpiredSessions removes expired sessions from the database
func (m *Manager) cleanupExpiredSessions() {
	_, err := m.db.Exec(`DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP`)
	if err != nil {
		log.Printf("Error cleaning up expired sessions: %v", err)
	}
}

// hashPassword creates a SHA-256 hash of the password
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// generateToken generates a secure random token
func generateToken() (string, error) {
	b := make([]byte, tokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// extractTokenFromRequest extracts the session token from the request cookie
func extractTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// Middleware returns an HTTP middleware that checks for valid sessions
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If auth is not enabled, pass through
		if !m.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Allow login endpoint without authentication
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Check for session token
		token := extractTokenFromRequest(r)
		if token == "" || !m.ValidateSession(token) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isPublicPath returns true if the path should be accessible without authentication
func isPublicPath(path string) bool {
	publicPaths := []string{
		"/api/auth/login",
		"/healthz",
		"/api/config",
	}

	for _, p := range publicPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}

	// Also allow static assets and the login page itself
	if strings.HasPrefix(path, "/assets/") {
		return true
	}

	return false
}

// SetSessionCookie sets the session cookie on the response
func SetSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Expires:  expiresAt,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookie clears the session cookie from the response
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
