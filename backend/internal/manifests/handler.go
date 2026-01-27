package manifests

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// MaxManifestSize is the maximum size for a manifest YAML content (1MB)
	MaxManifestSize = 1 * 1024 * 1024
)

// Handler handles HTTP requests for manifest operations
type Handler struct {
	db *sql.DB
}

// NewHandler creates a new manifests handler
func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// Manifest represents a saved manifest
type Manifest struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	YAMLContent string    `json:"yamlContent"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// CreateManifestRequest represents the request to create a manifest
type CreateManifestRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	YAMLContent string   `json:"yamlContent"`
	Tags        []string `json:"tags"`
}

// UpdateManifestRequest represents the request to update a manifest
type UpdateManifestRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	YAMLContent string   `json:"yamlContent"`
	Tags        []string `json:"tags"`
}

// ListManifests handles GET /api/manifests
func (h *Handler) ListManifests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := h.db.Query(`
		SELECT id, name, description, yaml_content, tags, created_at, updated_at
		FROM saved_manifests
		ORDER BY updated_at DESC
	`)
	if err != nil {
		log.Printf("Error querying manifests: %v", err)
		http.Error(w, "Failed to query manifests", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	manifests := []Manifest{}
	for rows.Next() {
		var m Manifest
		var tagsJSON sql.NullString
		err := rows.Scan(&m.ID, &m.Name, &m.Description, &m.YAMLContent, &tagsJSON, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning manifest row: %v", err)
			continue
		}

		// Parse tags JSON
		if tagsJSON.Valid && tagsJSON.String != "" {
			json.Unmarshal([]byte(tagsJSON.String), &m.Tags)
		}
		if m.Tags == nil {
			m.Tags = []string{}
		}

		manifests = append(manifests, m)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(manifests)
}

// GetManifest handles GET /api/manifests/:id
func (h *Handler) GetManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, err := h.extractID(r)
	if err != nil {
		http.Error(w, "Invalid manifest ID", http.StatusBadRequest)
		return
	}

	var m Manifest
	var tagsJSON sql.NullString
	err = h.db.QueryRow(`
		SELECT id, name, description, yaml_content, tags, created_at, updated_at
		FROM saved_manifests
		WHERE id = ?
	`, id).Scan(&m.ID, &m.Name, &m.Description, &m.YAMLContent, &tagsJSON, &m.CreatedAt, &m.UpdatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Manifest not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Error querying manifest: %v", err)
		http.Error(w, "Failed to query manifest", http.StatusInternalServerError)
		return
	}

	// Parse tags JSON
	if tagsJSON.Valid && tagsJSON.String != "" {
		json.Unmarshal([]byte(tagsJSON.String), &m.Tags)
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}

// CreateManifest handles POST /api/manifests
func (h *Handler) CreateManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateManifestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.YAMLContent == "" {
		http.Error(w, "yamlContent is required", http.StatusBadRequest)
		return
	}

	// Validate size
	if len(req.YAMLContent) > MaxManifestSize {
		http.Error(w, fmt.Sprintf("yamlContent exceeds maximum size of %d bytes", MaxManifestSize), http.StatusBadRequest)
		return
	}

	// Encode tags as JSON
	tagsJSON := "[]"
	if len(req.Tags) > 0 {
		tagsBytes, err := json.Marshal(req.Tags)
		if err == nil {
			tagsJSON = string(tagsBytes)
		}
	}

	// Check if name already exists
	var existingID int64
	err := h.db.QueryRow("SELECT id FROM saved_manifests WHERE name = ?", req.Name).Scan(&existingID)
	if err == nil {
		http.Error(w, "A manifest with this name already exists", http.StatusConflict)
		return
	}

	// Insert manifest
	result, err := h.db.Exec(`
		INSERT INTO saved_manifests (name, description, yaml_content, tags)
		VALUES (?, ?, ?, ?)
	`, req.Name, req.Description, req.YAMLContent, tagsJSON)

	if err != nil {
		log.Printf("Error creating manifest: %v", err)
		http.Error(w, "Failed to create manifest", http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()

	// Fetch the created manifest
	manifest, _ := h.getManifestByID(id)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(manifest)
}

// UpdateManifest handles PUT /api/manifests/:id
func (h *Handler) UpdateManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, err := h.extractID(r)
	if err != nil {
		http.Error(w, "Invalid manifest ID", http.StatusBadRequest)
		return
	}

	var req UpdateManifestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.YAMLContent == "" {
		http.Error(w, "yamlContent is required", http.StatusBadRequest)
		return
	}

	// Validate size
	if len(req.YAMLContent) > MaxManifestSize {
		http.Error(w, fmt.Sprintf("yamlContent exceeds maximum size of %d bytes", MaxManifestSize), http.StatusBadRequest)
		return
	}

	// Encode tags as JSON
	tagsJSON := "[]"
	if len(req.Tags) > 0 {
		tagsBytes, err := json.Marshal(req.Tags)
		if err == nil {
			tagsJSON = string(tagsBytes)
		}
	}

	// Check for duplicate name (if name changed)
	var existingID int64
	err = h.db.QueryRow("SELECT id FROM saved_manifests WHERE name = ? AND id != ?", req.Name, id).Scan(&existingID)
	if err == nil {
		http.Error(w, "A manifest with this name already exists", http.StatusConflict)
		return
	}

	// Update manifest
	result, err := h.db.Exec(`
		UPDATE saved_manifests
		SET name = ?, description = ?, yaml_content = ?, tags = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, req.Name, req.Description, req.YAMLContent, tagsJSON, id)

	if err != nil {
		log.Printf("Error updating manifest: %v", err)
		http.Error(w, "Failed to update manifest", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Manifest not found", http.StatusNotFound)
		return
	}

	// Fetch the updated manifest
	manifest, _ := h.getManifestByID(id)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(manifest)
}

// DeleteManifest handles DELETE /api/manifests/:id
func (h *Handler) DeleteManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, err := h.extractID(r)
	if err != nil {
		http.Error(w, "Invalid manifest ID", http.StatusBadRequest)
		return
	}

	result, err := h.db.Exec("DELETE FROM saved_manifests WHERE id = ?", id)
	if err != nil {
		log.Printf("Error deleting manifest: %v", err)
		http.Error(w, "Failed to delete manifest", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Manifest not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Manifest deleted successfully",
	})
}

// DownloadManifest handles GET /api/manifests/:id/download
func (h *Handler) DownloadManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, err := h.extractID(r)
	if err != nil {
		http.Error(w, "Invalid manifest ID", http.StatusBadRequest)
		return
	}

	var m Manifest
	var tagsJSON sql.NullString
	err = h.db.QueryRow(`
		SELECT id, name, description, yaml_content, tags, created_at, updated_at
		FROM saved_manifests
		WHERE id = ?
	`, id).Scan(&m.ID, &m.Name, &m.Description, &m.YAMLContent, &tagsJSON, &m.CreatedAt, &m.UpdatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Manifest not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Error querying manifest: %v", err)
		http.Error(w, "Failed to query manifest", http.StatusInternalServerError)
		return
	}

	// Set headers for file download
	filename := strings.ReplaceAll(m.Name, " ", "_") + ".yaml"
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "text/x-yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(m.YAMLContent))
}

// getManifestByID fetches a manifest by ID
func (h *Handler) getManifestByID(id int64) (*Manifest, error) {
	var m Manifest
	var tagsJSON sql.NullString
	err := h.db.QueryRow(`
		SELECT id, name, description, yaml_content, tags, created_at, updated_at
		FROM saved_manifests
		WHERE id = ?
	`, id).Scan(&m.ID, &m.Name, &m.Description, &m.YAMLContent, &tagsJSON, &m.CreatedAt, &m.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if tagsJSON.Valid && tagsJSON.String != "" {
		json.Unmarshal([]byte(tagsJSON.String), &m.Tags)
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}

	return &m, nil
}

// extractID extracts the manifest ID from the request URL path
// Expected path format: /api/manifests/:id or /api/manifests/:id/...
func (h *Handler) extractID(r *http.Request) (int64, error) {
	path := r.URL.Path
	// Remove prefix /api/manifests/
	prefix := "/api/manifests/"
	if !strings.HasPrefix(path, prefix) {
		return 0, fmt.Errorf("invalid path format")
	}

	suffix := path[len(prefix):]
	// Extract ID (stop at next slash if present)
	if idx := strings.Index(suffix, "/"); idx != -1 {
		suffix = suffix[:idx]
	}

	id, err := strconv.ParseInt(suffix, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid manifest ID: %w", err)
	}

	return id, nil
}

// RegisterRoutes registers the manifests routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// List and create
	mux.HandleFunc("/api/manifests", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.CreateManifest(w, r)
		} else {
			h.ListManifests(w, r)
		}
	})

	// Individual manifest operations
	manifestPath := "/api/manifests/"

	// Get, Update, Delete manifest by ID
	mux.HandleFunc(manifestPath, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, manifestPath) || r.URL.Path == manifestPath {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/download") {
			h.DownloadManifest(w, r)
		} else {
			switch r.Method {
			case http.MethodGet:
				h.GetManifest(w, r)
			case http.MethodPut, http.MethodPatch:
				h.UpdateManifest(w, r)
			case http.MethodDelete:
				h.DeleteManifest(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	})
}
