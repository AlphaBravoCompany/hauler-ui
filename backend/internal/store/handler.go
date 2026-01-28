package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
	"github.com/hauler-ui/hauler-ui/backend/internal/jobrunner"
)

// Handler handles HTTP requests for store operations
type Handler struct {
	JobRunner *jobrunner.Runner
	Cfg       *config.Config
}

// NewHandler creates a new store handler
func NewHandler(jobRunner *jobrunner.Runner, cfg *config.Config) *Handler {
	return &Handler{
		JobRunner: jobRunner,
		Cfg:       cfg,
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
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating add image job: %v", err)
		http.Error(w, "Failed to create add image job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.JobRunner.Start(r.Context(), job.ID); err != nil {
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
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating add chart job: %v", err)
		http.Error(w, "Failed to create add chart job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.JobRunner.Start(r.Context(), job.ID); err != nil {
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
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating add file job: %v", err)
		http.Error(w, "Failed to create add file job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.JobRunner.Start(r.Context(), job.ID); err != nil {
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
	if err := os.MkdirAll(h.Cfg.HaulerTempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create a temporary file with a predictable name for sync operations
	tempFile := filepath.Join(h.Cfg.HaulerTempDir, fmt.Sprintf("sync-manifest-%d.yaml", makeTimestamp()))
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
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating sync job: %v", err)
		http.Error(w, "Failed to create sync job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.JobRunner.Start(r.Context(), job.ID); err != nil {
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
		archivePath = filepath.Join(h.Cfg.DataDir, filename)
	}

	// Store metadata for post-job processing
	saveMetadata := map[string]interface{}{
		"filename":    filename,
		"archivePath": archivePath,
	}
	metadataJSON, _ := json.Marshal(saveMetadata)

	// Create a job with the metadata in env overrides for later retrieval
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, map[string]string{
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
	if err := h.JobRunner.Start(ctx, jobID); err != nil {
		log.Printf("Error starting save job %d: %v", jobID, err)
		return
	}

	// Wait a moment for the job to complete, then update result
	// In production, this would be better handled with a completion callback
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			job, err := h.JobRunner.GetJob(ctx, jobID)
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
					_ = h.JobRunner.UpdateResult(ctx, jobID, string(resultJSON))
				}
				return
			}

			if job.Status == jobrunner.StatusFailed {
				return
			}
		}
	}()
}

// HaulInfo represents information about a haul archive file
type HaulInfo struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// ListHauls handles GET /api/store/hauls
// Returns a list of .tar.zst archive files in the data directory
func (h *Handler) ListHauls(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Ensure data directory exists
	dataDir := h.Cfg.DataDir
	if dataDir == "" {
		dataDir = "."
	}

	// Read directory entries
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist yet, return empty list
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"hauls": []HaulInfo{},
			})
			return
		}
		log.Printf("Error reading data directory: %v", err)
		http.Error(w, "Failed to read data directory", http.StatusInternalServerError)
		return
	}

	// Filter for .tar.zst files
	var hauls []HaulInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Check for .tar.zst extension
		if strings.HasSuffix(strings.ToLower(name), ".tar.zst") {
			info, err := entry.Info()
			if err != nil {
				log.Printf("Error getting file info for %s: %v", name, err)
				continue
			}
			hauls = append(hauls, HaulInfo{
				Name:     name,
				Size:     info.Size(),
				Modified: info.ModTime(),
			})
		}
	}

	// Sort by modified time (newest first)
	// Sort in reverse order so newest is first
	for i := 0; i < len(hauls); i++ {
		for j := i + 1; j < len(hauls); j++ {
			if hauls[i].Modified.Before(hauls[j].Modified) {
				hauls[i], hauls[j] = hauls[j], hauls[i]
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"hauls": hauls,
	})
}

// DeleteHaul handles DELETE /api/store/hauls/{filename}
// Deletes a specific haul archive file
func (h *Handler) DeleteHaul(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from path
	// Path format: /api/store/hauls/{filename}
	prefix := "/api/store/hauls/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
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

	// Verify the file has .tar.zst extension before allowing delete
	if !strings.HasSuffix(strings.ToLower(filename), ".tar.zst") {
		http.Error(w, "Only .tar.zst files can be deleted through this endpoint", http.StatusBadRequest)
		return
	}

	// Build the full path to the archive
	archivePath := filepath.Join(h.Cfg.DataDir, filename)

	// Check if file exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Delete the file
	if err := os.Remove(archivePath); err != nil {
		log.Printf("Error deleting haul file %s: %v", archivePath, err)
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}

	log.Printf("Deleted haul archive: %s", archivePath)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Haul deleted successfully",
		"filename": filename,
	})
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
	archivePath := filepath.Join(h.Cfg.DataDir, filename)

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

// ExtractRequest represents the request to extract an artifact from the store
type ExtractRequest struct {
	ArtifactRef string `json:"artifactRef"`
	OutputDir   string `json:"outputDir,omitempty"`
}

// LoadRequest represents the request to load archives into the store
type LoadRequest struct {
	Filenames []string `json:"filenames,omitempty"`
	Clear      bool     `json:"clear"`
}

// Extract handles POST /api/store/extract
func (h *Handler) Extract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExtractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.ArtifactRef == "" {
		http.Error(w, "artifactRef is required", http.StatusBadRequest)
		return
	}

	// Build args for hauler store extract command
	args := []string{"store", "extract", req.ArtifactRef}

	// Optional output directory
	if req.OutputDir != "" {
		args = append(args, "--output", req.OutputDir)
	}

	// Create a job for the extract operation
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating extract job: %v", err)
		http.Error(w, "Failed to create extract job", http.StatusInternalServerError)
		return
	}

	// Start the job in background with result tracking
	go h.runExtractJob(r.Context(), job.ID, req.OutputDir)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":       job.ID,
		"message":     "Extract job started",
		"artifactRef": req.ArtifactRef,
		"outputDir":   req.OutputDir,
	})
}

// runExtractJob starts an extract job and updates the result with output directory on success
func (h *Handler) runExtractJob(ctx context.Context, jobID int64, outputDir string) {
	if err := h.JobRunner.Start(ctx, jobID); err != nil {
		log.Printf("Error starting extract job %d: %v", jobID, err)
		return
	}

	// Wait for job completion and update result
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			job, err := h.JobRunner.GetJob(ctx, jobID)
			if err != nil {
				return
			}

			if job.Status == jobrunner.StatusSucceeded {
				// If outputDir wasn't specified, try to determine it from the job output
				resultOutputDir := outputDir
				if resultOutputDir == "" {
					resultOutputDir = "." // Default to current directory
				}

				result := map[string]interface{}{
					"outputDir": resultOutputDir,
				}
				resultJSON, _ := json.Marshal(result)
				_ = h.JobRunner.UpdateResult(ctx, jobID, string(resultJSON))
				return
			}

			if job.Status == jobrunner.StatusFailed {
				return
			}
		}
	}()
}

// Load handles POST /api/store/load
func (h *Handler) Load(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Clear store if requested
	if req.Clear {
		if err := h.clearStore(ctx); err != nil {
			log.Printf("Error clearing store: %v", err)
			http.Error(w, "Failed to clear store: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Determine filenames to load
	filenames := req.Filenames
	if len(filenames) == 0 {
		filenames = []string{"haul.tar.zst"}
	}

	// Build args for hauler store load command
	args := []string{"store", "load"}
	for _, f := range filenames {
		args = append(args, "-f", f)
	}

	// Create a job for the load operation
	job, err := h.JobRunner.CreateJob(ctx, "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating load job: %v", err)
		http.Error(w, "Failed to create load job", http.StatusInternalServerError)
		return
	}

	// The job processor will start the queued job automatically.
	// Start a goroutine to track contents after completion.
	jobID := job.ID // Capture job ID for goroutine
	go func() {
		// Use background context since this runs after HTTP response is sent
		bgCtx := context.Background()
		// Wait for job to complete, then track contents
		for {
			time.Sleep(500 * time.Millisecond)
			j, err := h.JobRunner.GetJob(bgCtx, jobID)
			if err != nil {
				log.Printf("Error getting job status %d: %v", jobID, err)
				return
			}

			if j.Status == jobrunner.StatusSucceeded {
				for _, haulFile := range filenames {
					if err := h.trackStoreContents(bgCtx, haulFile); err != nil {
						log.Printf("Warning: failed to track contents for %s: %v", haulFile, err)
					}
				}
				return
			}

			if j.Status == jobrunner.StatusFailed {
				log.Printf("Load job %d failed, skipping tracking", jobID)
				return
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":     job.ID,
		"message":   "Load job started",
		"filenames": filenames,
		"cleared":    req.Clear,
	})
}

// CopyRequest represents the request to copy the store to a registry or directory
type CopyRequest struct {
	Target    string `json:"target"`
	Insecure  bool   `json:"insecure"`
	PlainHTTP bool   `json:"plainHttp"`
	Only      string `json:"only,omitempty"`
}

// RemoveRequest represents the request to remove artifacts from the store
type RemoveRequest struct {
	Match string `json:"match"`
	Force bool   `json:"force"`
}

// Copy handles POST /api/store/copy
func (h *Handler) Copy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CopyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Target == "" {
		http.Error(w, "target is required", http.StatusBadRequest)
		return
	}

	// Validate target format
	if !strings.HasPrefix(req.Target, "registry://") && !strings.HasPrefix(req.Target, "dir://") {
		http.Error(w, "target must start with registry:// or dir://", http.StatusBadRequest)
		return
	}

	// Build args for hauler store copy command
	args := []string{"store", "copy", req.Target}

	// Optional insecure flag
	if req.Insecure {
		args = append(args, "--insecure")
	}

	// Optional plain HTTP flag
	if req.PlainHTTP {
		args = append(args, "--plain-http")
	}

	// Optional only filter (sig, att)
	if req.Only != "" {
		args = append(args, "--only", req.Only)
	}

	// Create a job for the copy operation
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating copy job: %v", err)
		http.Error(w, "Failed to create copy job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.JobRunner.Start(r.Context(), job.ID); err != nil {
			log.Printf("Error starting copy job %d: %v", job.ID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":    job.ID,
		"message":  "Copy job started",
		"target":   req.Target,
	})
}

// Remove handles POST /api/store/remove
func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Match == "" {
		http.Error(w, "match is required", http.StatusBadRequest)
		return
	}

	// Build args for hauler store remove command
	args := []string{"store", "remove", req.Match}

	// Optional force flag to bypass confirmation
	if req.Force {
		args = append(args, "--force")
	}

	// Create a job for the remove operation
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating remove job: %v", err)
		http.Error(w, "Failed to create remove job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.JobRunner.Start(r.Context(), job.ID); err != nil {
			log.Printf("Error starting remove job %d: %v", job.ID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":    job.ID,
		"message":  "Remove job started",
		"match":    req.Match,
		"force":    req.Force,
	})
}

// StoreInfo represents the response from hauler store info
type StoreInfo struct {
	Images []ImageInfo  `json:"images"`
	Charts []ChartInfo  `json:"charts"`
	Files  []FileInfo   `json:"files"`
}

// ImageInfo represents information about a stored image
type ImageInfo struct {
	Name      string `json:"name,omitempty"`
	Digest    string `json:"digest,omitempty"`
	Size      int64  `json:"size,omitempty"`
	SourceHaul string `json:"sourceHaul,omitempty"`
}

// ChartInfo represents information about a stored chart
type ChartInfo struct {
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
	Digest    string `json:"digest,omitempty"`
	Size      int64  `json:"size,omitempty"`
	SourceHaul string `json:"sourceHaul,omitempty"`
}

// FileInfo represents information about a stored file
type FileInfo struct {
	Name      string `json:"name,omitempty"`
	Digest    string `json:"digest,omitempty"`
	Size      int64  `json:"size,omitempty"`
	SourceHaul string `json:"sourceHaul,omitempty"`
}

// StoreItem represents a single item from hauler store info raw output
type StoreItem struct {
	Reference string `json:"Reference"`
	Type      string `json:"Type"`
	Platform  string `json:"Platform"`
	Digest    string `json:"Digest"`
	Layers    int    `json:"Layers"`
	Size      int64  `json:"Size"`
}

// GetInfo handles GET /api/store/info
// Runs "hauler store info -o json" and returns parsed store contents
func (h *Handler) GetInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Build args for hauler store info command with JSON output
	args := []string{"store", "info", "-o", "json"}

	// Add store directory from config if available
	if h.Cfg.HaulerStoreDir != "" {
		args = append(args, "--store", h.Cfg.HaulerStoreDir)
	}

	// Run hauler store info command directly
	ctx := r.Context()
	cmd := exec.CommandContext(ctx, "hauler", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error running store info: %v, output: %s", err, string(output))
		http.Error(w, "Failed to get store info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch source_haul data from database
	type SourceInfo struct {
		Name      string
		SourceHaul string
	}
	// Map by digest (primary) and by name (fallback)
	digestSourceMap := make(map[string]string)
	nameSourceMap := make(map[string]string)

	db := h.JobRunner.DB()
	rows, err := db.QueryContext(ctx, `SELECT name, digest, source_haul FROM store_contents`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var s SourceInfo
			var digest sql.NullString
			if err := rows.Scan(&s.Name, &digest, &s.SourceHaul); err == nil {
				nameSourceMap[s.Name] = s.SourceHaul
				if digest.Valid {
					digestSourceMap[digest.String] = s.SourceHaul
				}
			}
		}
	}

	// Parse the array format from hauler store info
	var items []StoreItem
	storeInfo := StoreInfo{
		Images: []ImageInfo{},
		Charts: []ChartInfo{},
		Files:  []FileInfo{},
	}

	// Handle empty store (returns "null")
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "null" || trimmed == "" {
		// Empty store, keep default empty slices
	} else if err := json.Unmarshal(output, &items); err != nil {
		log.Printf("Error parsing store info JSON: %v, output: %s", err, string(output))
		http.Error(w, "Failed to parse store info: "+err.Error(), http.StatusInternalServerError)
		return
	} else {
		// Group items by type
		for _, item := range items {
			// Look up source_haul from database
			// Try digest first (most reliable), then exact name, then normalized name variations
			sourceHaul := digestSourceMap[item.Digest]
			if sourceHaul == "" {
				sourceHaul = nameSourceMap[item.Reference]
			}
			// For images, try matching without registry prefix as fallback
			if sourceHaul == "" {
				normalizedName := item.Reference
				// Strip common registry prefixes
				for _, prefix := range []string{"index.docker.io/", "docker.io/"} {
					if strings.HasPrefix(normalizedName, prefix) {
						normalizedName = strings.TrimPrefix(normalizedName, prefix)
						break
					}
				}
				if sourceHaul = nameSourceMap[normalizedName]; sourceHaul != "" {
					// Found it
				} else if sourceHaul = nameSourceMap["library/"+normalizedName]; sourceHaul != "" {
					// Try with library/ prefix for docker hub images
				}
			}

			switch strings.ToLower(item.Type) {
			case "image":
				storeInfo.Images = append(storeInfo.Images, ImageInfo{
					Name:      item.Reference,
					Digest:    item.Digest,
					Size:      item.Size,
					SourceHaul: sourceHaul,
				})
			case "chart":
				// Extract version from reference (format: hauler/chart:version)
				name := item.Reference
				version := ""
				if parts := strings.Split(name, ":"); len(parts) >= 2 {
					name = strings.Join(parts[:len(parts)-1], ":")
					version = parts[len(parts)-1]
				}
				storeInfo.Charts = append(storeInfo.Charts, ChartInfo{
					Name:       name,
					Version:    version,
					Digest:     item.Digest,
					Size:       item.Size,
					SourceHaul: sourceHaul,
				})
			case "file":
				storeInfo.Files = append(storeInfo.Files, FileInfo{
					Name:       item.Reference,
					Digest:     item.Digest,
					Size:       item.Size,
					SourceHaul: sourceHaul,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(storeInfo)
}

// clearStore removes the store directory and recreates the OCI layout structure
func (h *Handler) clearStore(ctx context.Context) error {
	storeDir := h.Cfg.HaulerStoreDir
	if storeDir == "" {
		storeDir = filepath.Join(h.Cfg.DataDir, "store")
	}

	// Ensure required directories exist
	tmpDir := filepath.Join(h.Cfg.DataDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("creating tmp directory: %w", err)
	}

	// Remove the store directory
	if err := os.RemoveAll(storeDir); err != nil {
		return fmt.Errorf("removing store directory: %w", err)
	}

	// Recreate the OCI layout structure
	blobsDir := filepath.Join(storeDir, "blobs", "sha256")
	if err := os.MkdirAll(blobsDir, 0755); err != nil {
		return fmt.Errorf("creating blobs directory: %w", err)
	}

	// Create minimal oci-layout
	ociLayout := []byte(`{"imageLayoutVersion": "1.0.0"}`)
	ociLayoutPath := filepath.Join(storeDir, "oci-layout")
	if err := os.WriteFile(ociLayoutPath, ociLayout, 0644); err != nil {
		return fmt.Errorf("creating oci-layout: %w", err)
	}

	log.Printf("Store cleared and recreated at %s", storeDir)
	return nil
}

// trackStoreContents adds entries to store_contents table for items from a haul
func (h *Handler) trackStoreContents(ctx context.Context, haulFilename string) error {
	storeDir := h.Cfg.HaulerStoreDir
	if storeDir == "" {
		storeDir = filepath.Join(h.Cfg.DataDir, "store")
	}

	// Parse the index.json to discover what's in the store
	indexPath := filepath.Join(storeDir, "index.json")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("reading index.json: %w", err)
	}

	var index struct {
		Manifests []struct {
			Digest      string                 `json:"digest"`
			Annotations map[string]interface{} `json:"annotations"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(indexData, &index); err != nil {
		return fmt.Errorf("parsing index.json: %w", err)
	}

	// Insert items into store_contents
	db := h.JobRunner.DB()
	for _, manifest := range index.Manifests {
		// Get name from annotations - prefer io.containerd.image.name (full reference)
		// over org.opencontainers.image.ref.name (short reference)
		var name string
		if manifest.Annotations != nil {
			// Try io.containerd.image.name first (has full registry prefix)
			if n, ok := manifest.Annotations["io.containerd.image.name"].(string); ok {
				name = n
			} else if n, ok := manifest.Annotations["org.opencontainers.image.ref.name"].(string); ok {
				name = n
			}
		}

		if name == "" {
			continue // Skip if we can't determine the name
		}

		// Determine content type from name
		contentType := "file"
		if strings.Contains(name, ":") {
			contentType = "image"
		} else if strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".tar.gz") {
			contentType = "chart"
		}

		_, err = db.ExecContext(ctx, `
			INSERT OR REPLACE INTO store_contents (content_type, name, digest, source_haul)
			VALUES (?, ?, ?, ?)
		`, contentType, name, manifest.Digest, haulFilename)

		if err != nil {
			log.Printf("Error inserting store content %s: %v", name, err)
		}
	}

	log.Printf("Tracked %d items from haul %s", len(index.Manifests), haulFilename)
	return nil
}

// rescanStore scans the store and rebuilds the store_contents table
func (h *Handler) rescanStore(ctx context.Context) (int, error) {
	storeDir := h.Cfg.HaulerStoreDir
	if storeDir == "" {
		storeDir = filepath.Join(h.Cfg.DataDir, "store")
	}

	// Clear existing store_contents
	db := h.JobRunner.DB()
	if _, err := db.ExecContext(ctx, "DELETE FROM store_contents"); err != nil {
		return 0, fmt.Errorf("clearing store_contents: %w", err)
	}

	// Scan blobs directory to discover content
	blobsDir := filepath.Join(storeDir, "blobs", "sha256")
	entries, err := os.ReadDir(blobsDir)
	if err != nil {
		return 0, fmt.Errorf("reading blobs directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Just count the blobs
		count++
	}

	// Parse index.json to get proper item names
	indexPath := filepath.Join(storeDir, "index.json")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return count, fmt.Errorf("reading index.json: %w", err)
	}

	var index struct {
		Manifests []struct {
			Name        string                 `json:"name"`
			Annotations map[string]interface{} `json:"annotations"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(indexData, &index); err != nil {
		return count, fmt.Errorf("parsing index.json: %w", err)
	}

	// Insert discovered items
	for _, manifest := range index.Manifests {
		contentType := "file"
		if strings.Contains(manifest.Name, ":") {
			contentType = "image"
		} else if strings.HasSuffix(manifest.Name, ".tgz") || strings.HasSuffix(manifest.Name, ".tar.gz") {
			contentType = "chart"
		}

		var digest string
		if manifest.Annotations != nil {
			if d, ok := manifest.Annotations["vnd.docker.reference.digest"].(string); ok {
				digest = d
			}
		}

		_, err = db.ExecContext(ctx, `
			INSERT OR IGNORE INTO store_contents (content_type, name, digest, source_haul, loaded_at)
			VALUES (?, ?, ?, NULL, datetime('now'))
		`, contentType, manifest.Name, digest)

		if err != nil {
			log.Printf("Error inserting store content %s: %v", manifest.Name, err)
		} else {
			count++
		}
	}

	log.Printf("Rescan complete: tracked %d items", count)
	return count, nil
}

// Rescan handles POST /api/store/rescan
func (h *Handler) Rescan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	count, err := h.rescanStore(r.Context())
	if err != nil {
		log.Printf("Error rescanning store: %v", err)
		http.Error(w, "Failed to rescan store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"itemsFound": count,
		"message":   fmt.Sprintf("Store rescanned, tracked %d items", count),
	})
}

// RegisterRoutes registers the store routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/store/info", h.GetInfo)
	mux.HandleFunc("/api/store/add-image", h.AddImage)
	mux.HandleFunc("/api/store/add-chart", h.AddChart)
	mux.HandleFunc("/api/store/add-file", h.AddFile)
	mux.HandleFunc("/api/store/sync", h.Sync)
	mux.HandleFunc("/api/store/save", h.Save)
	mux.HandleFunc("/api/store/load", h.Load)
	mux.HandleFunc("/api/store/extract", h.Extract)
	mux.HandleFunc("/api/store/copy", h.Copy)
	mux.HandleFunc("/api/store/remove", h.Remove)
	mux.HandleFunc("/api/store/rescan", h.Rescan)
	mux.HandleFunc("/api/store/hauls", h.ListHauls)
	mux.HandleFunc("/api/store/hauls/upload", h.UploadHaul)
	mux.HandleFunc("/api/store/hauls/", h.DeleteHaul)
	mux.HandleFunc("/api/downloads/", h.ServeDownload)
}

// UploadHaul handles POST /api/store/hauls/upload
// Accepts a .tar.zst file upload and saves it to the data directory
func (h *Handler) UploadHaul(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 100GB)
	if err := r.ParseMultipartForm(100 << 30); err != nil {
		log.Printf("Error parsing multipart form: %v", err)
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	// Get the file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error getting file from form: %v", err)
		http.Error(w, "No file provided or error reading file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := header.Filename

	// Validate filename has .tar.zst extension
	if !strings.HasSuffix(strings.ToLower(filename), ".tar.zst") {
		http.Error(w, "Only .tar.zst files are allowed", http.StatusBadRequest)
		return
	}

	// Security: ensure filename doesn't contain path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Ensure data directory exists
	dataDir := h.Cfg.DataDir
	if dataDir == "" {
		dataDir = "."
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("Error creating data directory: %v", err)
		http.Error(w, "Failed to create data directory", http.StatusInternalServerError)
		return
	}

	// Build the destination path
	destinationPath := filepath.Join(dataDir, filename)

	// Create the destination file
	destFile, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			http.Error(w, "A file with this name already exists", http.StatusConflict)
		} else {
			log.Printf("Error creating destination file: %v", err)
			http.Error(w, "Failed to create file", http.StatusInternalServerError)
		}
		return
	}
	defer destFile.Close()

	// Copy the uploaded file to destination
	written, err := io.Copy(destFile, file)
	if err != nil {
		log.Printf("Error copying file: %v", err)
		os.Remove(destinationPath)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	log.Printf("Uploaded haul archive: %s (%d bytes)", filename, written)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "File uploaded successfully",
		"filename": filename,
		"size":     written,
	})
}
