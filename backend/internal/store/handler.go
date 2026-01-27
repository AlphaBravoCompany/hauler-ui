package store

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
	"github.com/hauler-ui/hauler-ui/backend/internal/jobrunner"
)

// Handler handles HTTP requests for store operations
type Handler struct {
	jobRunner *jobrunner.Runner
	cfg       *config.Config
}

// NewHandler creates a new store handler
func NewHandler(jobRunner *jobrunner.Runner, cfg *config.Config) *Handler {
	return &Handler{
		jobRunner: jobRunner,
		cfg:       cfg,
	}
}

// AddImageRequest represents the request to add an image to the store
type AddImageRequest struct {
	ImageRef                    string `json:"imageRef"`
	Platform                    string `json:"platform,omitempty"`
	Key                         string `json:"key,omitempty"`
	CertificateIdentity         string `json:"certificateIdentity,omitempty"`
	CertificateIdentityRegexp   string `json:"certificateIdentityRegexp,omitempty"`
	CertificateOidcIssuer       string `json:"certificateOidcIssuer,omitempty"`
	CertificateOidcIssuerRegexp string `json:"certificateOidcIssuerRegexp,omitempty"`
	CertificateGithubWorkflow   string `json:"certificateGithubWorkflow,omitempty"`
	Rewrite                     string `json:"rewrite,omitempty"`
	UseTlogVerify               bool   `json:"useTlogVerify"`
}

// AddImage handles POST /api/store/add-image
func (h *Handler) AddImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AddImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.ImageRef == "" {
		http.Error(w, "imageRef is required", http.StatusBadRequest)
		return
	}

	// Build args for hauler store add image command
	args := []string{"store", "add", "image", req.ImageRef}

	// Optional platform
	if req.Platform != "" {
		args = append(args, "--platform", req.Platform)
	}

	// Optional key for signature verification
	if req.Key != "" {
		args = append(args, "--key", req.Key)
	}

	// Keyless options
	if req.CertificateIdentity != "" {
		args = append(args, "--certificate-identity", req.CertificateIdentity)
	}
	if req.CertificateIdentityRegexp != "" {
		args = append(args, "--certificate-identity-regexp", req.CertificateIdentityRegexp)
	}
	if req.CertificateOidcIssuer != "" {
		args = append(args, "--certificate-oidc-issuer", req.CertificateOidcIssuer)
	}
	if req.CertificateOidcIssuerRegexp != "" {
		args = append(args, "--certificate-oidc-issuer-regexp", req.CertificateOidcIssuerRegexp)
	}
	if req.CertificateGithubWorkflow != "" {
		args = append(args, "--certificate-github-workflow-repository", req.CertificateGithubWorkflow)
	}

	// Optional rewrite path
	if req.Rewrite != "" {
		args = append(args, "--rewrite", req.Rewrite)
	}

	// Optional tlog verify
	if req.UseTlogVerify {
		args = append(args, "--use-tlog-verify")
	}

	// Create a job for the add image operation
	job, err := h.jobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating add image job: %v", err)
		http.Error(w, "Failed to create add image job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.jobRunner.Start(r.Context(), job.ID); err != nil {
			log.Printf("Error starting add image job %d: %v", job.ID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":    job.ID,
		"message":  "Add image job started",
		"imageRef": req.ImageRef,
	})
}

// AddChartRequest represents the request to add a chart to the store
type AddChartRequest struct {
	Name                   string `json:"name"`
	RepoURL                string `json:"repoUrl,omitempty"`
	Version                string `json:"version,omitempty"`
	Username               string `json:"username,omitempty"`
	Password               string `json:"password,omitempty"`
	KeyFile                string `json:"keyFile,omitempty"`
	CertFile               string `json:"certFile,omitempty"`
	CAFile                 string `json:"caFile,omitempty"`
	InsecureSkipTLSVerify  bool   `json:"insecureSkipTlsVerify"`
	PlainHTTP              bool   `json:"plainHttp"`
	Verify                 bool   `json:"verify"`
	AddDependencies        bool   `json:"addDependencies"`
	AddImages              bool   `json:"addImages"`
}

// AddChart handles POST /api/store/add-chart
func (h *Handler) AddChart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AddChartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	// Build args for hauler store add chart command
	args := []string{"store", "add", "chart", req.Name}

	// Optional repo URL
	if req.RepoURL != "" {
		args = append(args, "--repo", req.RepoURL)
	}

	// Optional version
	if req.Version != "" {
		args = append(args, "--version", req.Version)
	}

	// Optional username/password for auth
	if req.Username != "" {
		args = append(args, "--username", req.Username)
	}
	if req.Password != "" {
		args = append(args, "--password", req.Password)
	}

	// Optional TLS files
	if req.KeyFile != "" {
		args = append(args, "--key-file", req.KeyFile)
	}
	if req.CertFile != "" {
		args = append(args, "--cert-file", req.CertFile)
	}
	if req.CAFile != "" {
		args = append(args, "--ca-file", req.CAFile)
	}

	// TLS options
	if req.InsecureSkipTLSVerify {
		args = append(args, "--insecure-skip-tls-verify")
	}
	if req.PlainHTTP {
		args = append(args, "--plain-http")
	}

	// Verify option
	if req.Verify {
		args = append(args, "--verify")
	}

	// Capability-driven options
	if req.AddDependencies {
		args = append(args, "--add-dependencies")
	}
	if req.AddImages {
		args = append(args, "--add-images")
	}

	// Create a job for the add chart operation
	job, err := h.jobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating add chart job: %v", err)
		http.Error(w, "Failed to create add chart job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.jobRunner.Start(r.Context(), job.ID); err != nil {
			log.Printf("Error starting add chart job %d: %v", job.ID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":   job.ID,
		"message": "Add chart job started",
		"name":    req.Name,
	})
}

// AddFileRequest represents the request to add a file to the store
type AddFileRequest struct {
	FilePath string `json:"filePath,omitempty"`
	URL      string `json:"url,omitempty"`
	Name     string `json:"name,omitempty"`
}

// AddFile handles POST /api/store/add-file
func (h *Handler) AddFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AddFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate that either filePath or URL is provided (mutually exclusive)
	if req.FilePath == "" && req.URL == "" {
		http.Error(w, "Either filePath or url is required", http.StatusBadRequest)
		return
	}
	if req.FilePath != "" && req.URL != "" {
		http.Error(w, "Please provide either filePath or url, not both", http.StatusBadRequest)
		return
	}

	// Determine the file source
	fileSource := req.FilePath
	if fileSource == "" {
		fileSource = req.URL
	}

	// Build args for hauler store add file command
	args := []string{"store", "add", "file", fileSource}

	// Optional name rewrite
	if req.Name != "" {
		args = append(args, "--name", req.Name)
	}

	// Create a job for the add file operation
	job, err := h.jobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating add file job: %v", err)
		http.Error(w, "Failed to create add file job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.jobRunner.Start(r.Context(), job.ID); err != nil {
			log.Printf("Error starting add file job %d: %v", job.ID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":   job.ID,
		"message": "Add file job started",
		"file":    fileSource,
	})
}

// RegisterRoutes registers the store routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/store/add-image", h.AddImage)
	mux.HandleFunc("/api/store/add-chart", h.AddChart)
	mux.HandleFunc("/api/store/add-file", h.AddFile)
}
