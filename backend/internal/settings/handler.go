package settings

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Setting represents a single setting in the database
type Setting struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// SettingsResponse represents the response for settings API
type SettingsResponse struct {
	Settings          map[string]Setting `json:"settings"`
	LogLevel          string             `json:"logLevel"`
	Retries           string             `json:"retries"`
	IgnoreErrors      string             `json:"ignoreErrors"`
	DefaultPlatform   string             `json:"defaultPlatform"`
	DefaultKeyPath    string             `json:"defaultKeyPath"`
	TempDir           string             `json:"tempDir"`
	EnvHelp           map[string]string  `json:"envHelp"`
}

// Handler handles HTTP requests for settings operations
type Handler struct {
	db *sql.DB
}

// NewHandler creates a new settings handler
func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// GetSettings retrieves all settings from the database
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Query all settings
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT key, value, description, updated_at FROM settings ORDER BY key`,
	)
	if err != nil {
		log.Printf("Error querying settings: %v", err)
		http.Error(w, "Failed to query settings", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	settingsMap := make(map[string]Setting)
	for rows.Next() {
		var s Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.Description, &s.UpdatedAt); err != nil {
			log.Printf("Error scanning setting: %v", err)
			continue
		}
		settingsMap[s.Key] = s
	}

	// Build response with known settings
	response := SettingsResponse{
		Settings: settingsMap,
		EnvHelp: map[string]string{
			"log_level":         "HAULER_LOG_LEVEL",
			"retries":           "HAULER_RETRIES",
			"ignore_errors":     "HAULER_IGNORE_ERRORS",
			"default_platform":  "HAULER_DEFAULT_PLATFORM",
			"default_key_path":  "HAULER_KEY_PATH",
			"temp_dir":          "HAULER_TEMP_DIR",
		},
	}

	// Set individual fields for convenience
	if s, ok := settingsMap["log_level"]; ok {
		response.LogLevel = s.Value
	}
	if s, ok := settingsMap["retries"]; ok {
		response.Retries = s.Value
	}
	if s, ok := settingsMap["ignore_errors"]; ok {
		response.IgnoreErrors = s.Value
	}
	if s, ok := settingsMap["default_platform"]; ok {
		response.DefaultPlatform = s.Value
	}
	if s, ok := settingsMap["default_key_path"]; ok {
		response.DefaultKeyPath = s.Value
	}
	if s, ok := settingsMap["temp_dir"]; ok {
		response.TempDir = s.Value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// UpdateSettingsRequest represents the request to update settings
type UpdateSettingsRequest struct {
	LogLevel        string `json:"logLevel"`
	Retries         string `json:"retries"`
	IgnoreErrors    string `json:"ignoreErrors"`
	DefaultPlatform string `json:"defaultPlatform"`
	DefaultKeyPath  string `json:"defaultKeyPath"`
	TempDir         string `json:"tempDir"`
}

// UpdateSettings updates settings in the database
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Update each setting if provided
	settingsToUpdate := map[string]string{
		"log_level":        req.LogLevel,
		"retries":          req.Retries,
		"ignore_errors":    req.IgnoreErrors,
		"default_platform": req.DefaultPlatform,
		"default_key_path": req.DefaultKeyPath,
		"temp_dir":         req.TempDir,
	}

	for key, value := range settingsToUpdate {
		// Only update non-empty values
		if value != "" {
			_, err := h.db.ExecContext(r.Context(),
				`INSERT INTO settings (key, value, updated_at)
				 VALUES (?, ?, CURRENT_TIMESTAMP)
				 ON CONFLICT (key) DO UPDATE SET
				 value = excluded.value,
				 updated_at = CURRENT_TIMESTAMP`,
				key, value,
			)
			if err != nil {
				log.Printf("Error updating setting %s: %v", key, err)
				http.Error(w, "Failed to update setting "+key, http.StatusInternalServerError)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Settings updated successfully",
	})
}

// RegisterRoutes registers the settings routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetSettings(w, r)
		case http.MethodPut:
			h.UpdateSettings(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
