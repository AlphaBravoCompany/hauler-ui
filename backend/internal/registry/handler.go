package registry

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
	"github.com/hauler-ui/hauler-ui/backend/internal/jobrunner"
)

// Handler handles HTTP requests for registry operations
type Handler struct {
	jobRunner *jobrunner.Runner
	cfg       *config.Config
}

// NewHandler creates a new registry handler
func NewHandler(jobRunner *jobrunner.Runner, cfg *config.Config) *Handler {
	return &Handler{
		jobRunner: jobRunner,
		cfg:       cfg,
	}
}

// LoginRequest represents the request to login to a registry
type LoginRequest struct {
	Registry string `json:"registry"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// LogoutRequest represents the request to logout from a registry
type LogoutRequest struct {
	Registry string `json:"registry"`
}

// Login handles POST /api/registry/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Registry == "" {
		http.Error(w, "registry is required", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return
	}

	// Build args for hauler login command
	args := []string{"login", req.Registry}

	// Set environment variables for username and password
	// This is more secure than passing as command line args
	envOverrides := map[string]string{
		"HAULER_REGISTRY_USERNAME": req.Username,
		"HAULER_REGISTRY_PASSWORD": req.Password,
	}

	// Create a job for the login operation
	job, err := h.jobRunner.CreateJob(r.Context(), "hauler", args, envOverrides)
	if err != nil {
		log.Printf("Error creating login job: %v", err)
		http.Error(w, "Failed to create login job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.jobRunner.Start(r.Context(), job.ID); err != nil {
			log.Printf("Error starting login job %d: %v", job.ID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":       job.ID,
		"message":     "Login job started",
		"registry":    req.Registry,
		"username":    req.Username,
	})
}

// Logout handles POST /api/registry/logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Registry == "" {
		http.Error(w, "registry is required", http.StatusBadRequest)
		return
	}

	// Build args for hauler logout command
	args := []string{"logout", req.Registry}

	// Create a job for the logout operation
	job, err := h.jobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating logout job: %v", err)
		http.Error(w, "Failed to create logout job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.jobRunner.Start(r.Context(), job.ID); err != nil {
			log.Printf("Error starting logout job %d: %v", job.ID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":    job.ID,
		"message":  "Logout job started",
		"registry": req.Registry,
	})
}

// Info handles GET /api/registry/info
// Returns information about configured registries
func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the config handler to return registry info
	info := map[string]interface{}{
		"dockerAuthPath":     h.cfg.DockerAuthPath,
		"homeDir":            h.cfg.HaulerDir,
		"credentialsMessage": "Credentials are stored by hauler in the Docker config.json file",
	}

	// Parse the docker auth path to show user-friendly location
	displayPath := h.cfg.DockerAuthPath
	if strings.Contains(displayPath, "/data/.docker/config.json") {
		displayPath = "~/.docker/config.json (mapped to /data/.docker/config.json in container)"
	}

	info["displayPath"] = displayPath

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

// RegisterRoutes registers the registry routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/registry/login", h.Login)
	mux.HandleFunc("/api/registry/logout", h.Logout)
	mux.HandleFunc("/api/registry/info", h.Info)
}
