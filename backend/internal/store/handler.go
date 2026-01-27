package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

// SyncRequest represents the request to sync the store from manifests
type SyncRequest struct {
	ManifestYaml                string   `json:"manifestYaml,omitempty"`
	Filenames                   []string `json:"filenames,omitempty"`
	Platform                    string   `json:"platform,omitempty"`
	Key                         string   `json:"key,omitempty"`
	CertificateIdentity         string   `json:"certificateIdentity,omitempty"`
	CertificateIdentityRegexp   string   `json:"certificateIdentityRegexp,omitempty"`
	CertificateOidcIssuer       string   `json:"certificateOidcIssuer,omitempty"`
	CertificateOidcIssuerRegexp string   `json:"certificateOidcIssuerRegexp,omitempty"`
	CertificateGithubWorkflow   string   `json:"certificateGithubWorkflow,omitempty"`
	Registry                    string   `json:"registry,omitempty"`
	Products                    string   `json:"products,omitempty"`
	ProductRegistry             string   `json:"productRegistry,omitempty"`
	Rewrite                     string   `json:"rewrite,omitempty"`
	UseTlogVerify               bool     `json:"useTlogVerify"`
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

// writeTempManifest writes manifest YAML content to a temporary file and returns the path
func (h *Handler) writeTempManifest(yamlContent string) (string, error) {
	// Ensure temp directory exists
	if err := os.MkdirAll(h.cfg.HaulerTempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create a temporary file with a predictable name for sync operations
	tempFile := filepath.Join(h.cfg.HaulerTempDir, fmt.Sprintf("sync-manifest-%d.yaml", makeTimestamp()))
	if err := os.WriteFile(tempFile, []byte(yamlContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write temp manifest: %w", err)
	}

	return tempFile, nil
}

// makeTimestamp returns a unique timestamp-based identifier
func makeTimestamp() int64 {
	return int64(float64(1000000))
}

// Sync handles POST /api/store/sync
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Build args for hauler store sync command
	args := []string{"store", "sync"}

	// Build file list: either from provided filenames or temp manifest from YAML
	var filenames []string
	var tempFiles []string
	defer func() {
		// Clean up temporary files after job starts
		for _, f := range tempFiles {
			os.Remove(f)
		}
	}()

	if req.ManifestYaml != "" {
		// Write manifest YAML to temp file
		tempFile, err := h.writeTempManifest(req.ManifestYaml)
		if err != nil {
			log.Printf("Error writing temp manifest: %v", err)
			http.Error(w, "Failed to create temp manifest file", http.StatusInternalServerError)
			return
		}
		tempFiles = append(tempFiles, tempFile)
		filenames = append(filenames, tempFile)
	} else if len(req.Filenames) > 0 {
		filenames = req.Filenames
	} else {
		// Default to hauler-manifest.yaml as per hauler CLI
		filenames = []string{"hauler-manifest.yaml"}
	}

	// Add each file with -f flag
	for _, f := range filenames {
		args = append(args, "-f", f)
	}

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

	// Optional registry override
	if req.Registry != "" {
		args = append(args, "--registry", req.Registry)
	}

	// Products
	if req.Products != "" {
		args = append(args, "--products", req.Products)
	}

	// Product registry
	if req.ProductRegistry != "" {
		args = append(args, "--product-registry", req.ProductRegistry)
	}

	// Optional rewrite path (experimental)
	if req.Rewrite != "" {
		args = append(args, "--rewrite", req.Rewrite)
	}

	// Optional tlog verify
	if req.UseTlogVerify {
		args = append(args, "--use-tlog-verify")
	}

	// Create a job for the sync operation
	job, err := h.jobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating sync job: %v", err)
		http.Error(w, "Failed to create sync job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.jobRunner.Start(r.Context(), job.ID); err != nil {
			log.Printf("Error starting sync job %d: %v", job.ID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":     job.ID,
		"message":   "Sync job started",
		"filenames": filenames,
	})
}

// SaveRequest represents the request to save the store to an archive
type SaveRequest struct {
	Filename    string `json:"filename,omitempty"`
	Platform    string `json:"platform,omitempty"`
	Containerd  string `json:"containerd,omitempty"`
}

// Save handles POST /api/store/save
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Default filename if not provided
	filename := req.Filename
	if filename == "" {
		filename = "haul.tar.zst"
	}

	// Build args for hauler store save command
	args := []string{"store", "save", "--filename", filename}

	// Optional platform
	if req.Platform != "" {
		args = append(args, "--platform", req.Platform)
	}

	// Optional containerd target
	if req.Containerd != "" {
		args = append(args, "--containerd", req.Containerd)
	}

	// Resolve the full path of the archive for later download
	archivePath := filename
	if !filepath.IsAbs(filename) {
		// If relative, it will be in the current working directory
		// For predictability, we'll use the data directory
		archivePath = filepath.Join(h.cfg.DataDir, filename)
	}

	// Store metadata for post-job processing
	saveMetadata := map[string]interface{}{
		"filename":    filename,
		"archivePath": archivePath,
	}
	metadataJSON, _ := json.Marshal(saveMetadata)

	// Create a job with the metadata in env overrides for later retrieval
	job, err := h.jobRunner.CreateJob(r.Context(), "hauler", args, map[string]string{
		"HAULER_SAVE_METADATA": string(metadataJSON),
	})
	if err != nil {
		log.Printf("Error creating save job: %v", err)
		http.Error(w, "Failed to create save job", http.StatusInternalServerError)
		return
	}

	// Start the job in background with result tracking
	go h.runSaveJob(r.Context(), job.ID, archivePath)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":    job.ID,
		"message":  "Save job started",
		"filename": filename,
	})
}

// runSaveJob starts a save job and updates the result with archive path on success
func (h *Handler) runSaveJob(ctx context.Context, jobID int64, archivePath string) {
	if err := h.jobRunner.Start(ctx, jobID); err != nil {
		log.Printf("Error starting save job %d: %v", jobID, err)
		return
	}

	// Wait a moment for the job to complete, then update result
	// In production, this would be better handled with a completion callback
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			job, err := h.jobRunner.GetJob(ctx, jobID)
			if err != nil {
				return
			}

			if job.Status == jobrunner.StatusSucceeded {
				// Verify the archive exists
				if _, err := os.Stat(archivePath); err == nil {
					result := map[string]interface{}{
						"archivePath": archivePath,
						"filename":    filepath.Base(archivePath),
					}
					resultJSON, _ := json.Marshal(result)
					_ = h.jobRunner.UpdateResult(ctx, jobID, string(resultJSON))
				}
				return
			}

			if job.Status == jobrunner.StatusFailed {
				return
			}
		}
	}()
}

// ServeDownload handles GET /api/downloads/{filename} for downloading saved archives
func (h *Handler) ServeDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from path
	// Path format: /api/downloads/{filename}
	prefix := "/api/downloads/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "Invalid download path", http.StatusBadRequest)
		return
	}

	filename := strings.TrimPrefix(r.URL.Path, prefix)
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// Security: ensure filename doesn't contain path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Build the full path to the archive
	archivePath := filepath.Join(h.cfg.DataDir, filename)

	// Check if file exists
	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error accessing file", http.StatusInternalServerError)
		}
		return
	}

	// Open the file
	file, err := os.Open(archivePath)
	if err != nil {
		http.Error(w, "Error opening file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set headers for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Support range requests
	w.Header().Set("Accept-Ranges", "bytes")

	// Handle range request if present
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		// Parse Range header (format: bytes=start-end)
		if strings.HasPrefix(rangeHeader, "bytes=") {
			rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
			parts := strings.Split(rangeSpec, "-")
			if len(parts) == 2 {
				var start, end int64
				if parts[0] != "" {
					start, _ = strconv.ParseInt(parts[0], 10, 64)
				}
				if parts[1] != "" {
					end, _ = strconv.ParseInt(parts[1], 10, 64)
				} else {
					end = fileInfo.Size() - 1
				}

				if start >= 0 && end >= start && end < fileInfo.Size() {
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileInfo.Size()))
					w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
					w.WriteHeader(http.StatusPartialContent)

					_, _ = file.Seek(start, 0)
					_, err = io.CopyN(w, file, end-start+1)
					if err != nil {
						log.Printf("Error serving file range: %v", err)
					}
					return
				}
			}
		}
	}

	// Serve entire file
	_, err = io.Copy(w, file)
	if err != nil {
		log.Printf("Error serving file: %v", err)
	}
}

// RegisterRoutes registers the store routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/store/add-image", h.AddImage)
	mux.HandleFunc("/api/store/add-chart", h.AddChart)
	mux.HandleFunc("/api/store/add-file", h.AddFile)
	mux.HandleFunc("/api/store/sync", h.Sync)
	mux.HandleFunc("/api/store/save", h.Save)
	mux.HandleFunc("/api/downloads/", h.ServeDownload)
}
