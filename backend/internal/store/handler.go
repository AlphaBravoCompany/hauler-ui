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
	"github.com/hauler-ui/hauler-ui/backend/internal/hauls"
	"github.com/hauler-ui/hauler-ui/backend/internal/jobrunner"
)

// Handler handles HTTP requests for store operations
type Handler struct {
	JobRunner *jobrunner.Runner
	Cfg       *config.Config
	Hauls     *hauls.Service
}

// NewHandler creates a new store handler
func NewHandler(jobRunner *jobrunner.Runner, cfg *config.Config, haulSvc *hauls.Service) *Handler {
	return &Handler{
		JobRunner: jobRunner,
		Cfg:       cfg,
		Hauls:     haulSvc,
	}
}

// resolveHaul returns the haul a request targets. When haulID is 0 it falls back
// to the default haul. It also returns the "--store <dir>" args that scope a
// hauler command to that haul's isolated store directory.
func (h *Handler) resolveHaul(ctx context.Context, haulID int64) (*hauls.Haul, []string, error) {
	var (
		haul *hauls.Haul
		err  error
	)
	if haulID > 0 {
		haul, err = h.Hauls.Get(ctx, haulID)
	} else {
		haul, err = h.Hauls.EnsureDefault(ctx)
	}
	if err != nil {
		return nil, nil, err
	}
	return haul, []string{"--store", haul.StoreDir}, nil
}

// tagJobHaul records which haul a job operated on, so job history can be filtered.
func (h *Handler) tagJobHaul(ctx context.Context, jobID, haulID int64) {
	if _, err := h.JobRunner.DB().ExecContext(ctx, `UPDATE jobs SET haul_id = ? WHERE id = ?`, haulID, jobID); err != nil {
		log.Printf("Warning: failed to tag job %d with haul %d: %v", jobID, haulID, err)
	}
}

// AddImageRequest represents the request to add an image to the store
type AddImageRequest struct {
	HaulID                      int64  `json:"haulId,omitempty"`
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

	haul, storeArgs, err := h.resolveHaul(r.Context(), req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
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

	// Scope to this haul's store.
	args = append(args, storeArgs...)

	// Create a job for the add image operation
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating add image job: %v", err)
		http.Error(w, "Failed to create add image job", http.StatusInternalServerError)
		return
	}
	h.tagJobHaul(r.Context(), job.ID, haul.ID)
	go h.trackAfterJob(job.ID, haul)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":    job.ID,
		"message":  "Add image job started",
		"imageRef": req.ImageRef,
		"haulId":   haul.ID,
	})
}

// AddChartRequest represents the request to add a chart to the store
type AddChartRequest struct {
	HaulID                 int64  `json:"haulId,omitempty"`
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

	haul, storeArgs, err := h.resolveHaul(r.Context(), req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
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

	// Scope to this haul's store.
	args = append(args, storeArgs...)

	// Create a job for the add chart operation
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating add chart job: %v", err)
		http.Error(w, "Failed to create add chart job", http.StatusInternalServerError)
		return
	}
	h.tagJobHaul(r.Context(), job.ID, haul.ID)
	go h.trackAfterJob(job.ID, haul)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":   job.ID,
		"message": "Add chart job started",
		"name":    req.Name,
		"haulId":  haul.ID,
	})
}

// AddFileRequest represents the request to add a file to the store
type AddFileRequest struct {
	HaulID   int64  `json:"haulId,omitempty"`
	FilePath string `json:"filePath,omitempty"`
	URL      string `json:"url,omitempty"`
	Name     string `json:"name,omitempty"`
}

// SyncRequest represents the request to sync the store from manifests
type SyncRequest struct {
	HaulID                      int64    `json:"haulId,omitempty"`
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

	haul, storeArgs, err := h.resolveHaul(r.Context(), req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
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

	// Scope to this haul's store.
	args = append(args, storeArgs...)

	// Create a job for the add file operation
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating add file job: %v", err)
		http.Error(w, "Failed to create add file job", http.StatusInternalServerError)
		return
	}
	h.tagJobHaul(r.Context(), job.ID, haul.ID)
	go h.trackAfterJob(job.ID, haul)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":   job.ID,
		"message": "Add file job started",
		"file":    fileSource,
		"haulId":  haul.ID,
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

	haul, storeArgs, err := h.resolveHaul(r.Context(), req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
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

	// Scope to this haul's store.
	args = append(args, storeArgs...)

	// Create a job for the sync operation
	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating sync job: %v", err)
		http.Error(w, "Failed to create sync job", http.StatusInternalServerError)
		return
	}
	h.tagJobHaul(r.Context(), job.ID, haul.ID)
	go h.trackAfterJob(job.ID, haul)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":     job.ID,
		"message":   "Sync job started",
		"filenames": filenames,
		"haulId":    haul.ID,
	})
}

// SaveRequest represents the request to save the store to an archive
type SaveRequest struct {
	HaulID     int64  `json:"haulId,omitempty"`
	Filename   string `json:"filename,omitempty"`
	Platform   string `json:"platform,omitempty"`
	Containerd string `json:"containerd,omitempty"`
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

	haul, storeArgs, err := h.resolveHaul(r.Context(), req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Default filename if not provided. Archives are written into the haul's
	// own archives directory so they stay associated with the haul.
	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		filename = haul.Slug + ".tar.zst"
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".tar.zst") {
		filename += ".tar.zst"
	}
	// Reject path traversal in user-supplied filenames.
	if strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(haul.ArchivesDir(), 0755); err != nil {
		http.Error(w, "Failed to prepare archives directory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	archivePath := filepath.Join(haul.ArchivesDir(), filename)

	// Build args for hauler store save command
	args := []string{"store", "save", "--filename", archivePath}

	// Optional platform
	if req.Platform != "" {
		args = append(args, "--platform", req.Platform)
	}

	// Optional containerd target
	if req.Containerd != "" {
		args = append(args, "--containerd", req.Containerd)
	}

	// Scope to this haul's store.
	args = append(args, storeArgs...)

	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating save job: %v", err)
		http.Error(w, "Failed to create save job", http.StatusInternalServerError)
		return
	}
	h.tagJobHaul(r.Context(), job.ID, haul.ID)

	// Track the archive path and download URL once the job succeeds.
	go h.trackSaveResult(job.ID, haul.ID, archivePath, filename)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":    job.ID,
		"message":  "Save job started",
		"filename": filename,
		"haulId":   haul.ID,
	})
}

// trackSaveResult waits for a save job to finish and records the resulting
// archive path and download URL on the job. Uses a background context because
// it outlives the originating HTTP request.
func (h *Handler) trackSaveResult(jobID, haulID int64, archivePath, filename string) {
	ctx := context.Background()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		job, err := h.JobRunner.GetJob(ctx, jobID)
		if err != nil {
			return
		}
		switch job.Status {
		case jobrunner.StatusSucceeded:
			if _, err := os.Stat(archivePath); err == nil {
				result := map[string]interface{}{
					"archivePath": archivePath,
					"filename":    filename,
					"downloadUrl": fmt.Sprintf("/api/hauls/%d/archives/%s", haulID, filename),
				}
				resultJSON, _ := json.Marshal(result)
				_ = h.JobRunner.UpdateResult(ctx, jobID, string(resultJSON))
			}
			return
		case jobrunner.StatusFailed:
			return
		}
	}
}

// ExtractRequest represents the request to extract an artifact from the store
type ExtractRequest struct {
	HaulID      int64  `json:"haulId,omitempty"`
	ArtifactRef string `json:"artifactRef"`
	OutputDir   string `json:"outputDir,omitempty"`
}

// LoadRequest represents the request to load archives into the store
type LoadRequest struct {
	HaulID    int64    `json:"haulId,omitempty"`
	Filenames []string `json:"filenames,omitempty"`
	Clear     bool     `json:"clear"`
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

	haul, storeArgs, err := h.resolveHaul(r.Context(), req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Build args for hauler store extract command
	args := []string{"store", "extract", req.ArtifactRef}

	// Optional output directory
	if req.OutputDir != "" {
		args = append(args, "--output", req.OutputDir)
	}

	// Scope to this haul's store.
	args = append(args, storeArgs...)

	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating extract job: %v", err)
		http.Error(w, "Failed to create extract job", http.StatusInternalServerError)
		return
	}
	h.tagJobHaul(r.Context(), job.ID, haul.ID)

	// Record the output directory on success.
	go h.trackExtractResult(job.ID, req.OutputDir)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":       job.ID,
		"message":     "Extract job started",
		"artifactRef": req.ArtifactRef,
		"outputDir":   req.OutputDir,
		"haulId":      haul.ID,
	})
}

// trackExtractResult records the output directory once an extract job succeeds.
func (h *Handler) trackExtractResult(jobID int64, outputDir string) {
	ctx := context.Background()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		job, err := h.JobRunner.GetJob(ctx, jobID)
		if err != nil {
			return
		}
		switch job.Status {
		case jobrunner.StatusSucceeded:
			resultOutputDir := outputDir
			if resultOutputDir == "" {
				resultOutputDir = "."
			}
			resultJSON, _ := json.Marshal(map[string]interface{}{"outputDir": resultOutputDir})
			_ = h.JobRunner.UpdateResult(ctx, jobID, string(resultJSON))
			return
		case jobrunner.StatusFailed:
			return
		}
	}
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

	haul, storeArgs, err := h.resolveHaul(ctx, req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Clear this haul's store if requested.
	if req.Clear {
		if err := h.clearStore(haul.StoreDir); err != nil {
			log.Printf("Error clearing store: %v", err)
			http.Error(w, "Failed to clear store: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := h.JobRunner.DB().ExecContext(ctx, `DELETE FROM store_contents WHERE haul_id = ?`, haul.ID); err != nil {
			log.Printf("Warning: failed to clear tracked contents for haul %d: %v", haul.ID, err)
		}
	}

	// Determine archives to load. Bare filenames are resolved against the haul's
	// archives directory; absolute paths are used as-is.
	filenames := req.Filenames
	if len(filenames) == 0 {
		http.Error(w, "at least one filename is required", http.StatusBadRequest)
		return
	}
	resolved := make([]string, 0, len(filenames))
	for _, f := range filenames {
		if filepath.IsAbs(f) {
			resolved = append(resolved, f)
		} else {
			resolved = append(resolved, filepath.Join(haul.ArchivesDir(), f))
		}
	}

	// Build args for hauler store load command
	args := []string{"store", "load"}
	for _, f := range resolved {
		args = append(args, "-f", f)
	}
	args = append(args, storeArgs...)

	job, err := h.JobRunner.CreateJob(ctx, "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating load job: %v", err)
		http.Error(w, "Failed to create load job", http.StatusInternalServerError)
		return
	}
	h.tagJobHaul(ctx, job.ID, haul.ID)

	// After the load completes, track what landed in the store for this haul.
	jobID := job.ID
	go func() {
		bgCtx := context.Background()
		for {
			time.Sleep(500 * time.Millisecond)
			j, err := h.JobRunner.GetJob(bgCtx, jobID)
			if err != nil {
				log.Printf("Error getting job status %d: %v", jobID, err)
				return
			}
			if j.Status == jobrunner.StatusSucceeded {
				for _, f := range filenames {
					if err := h.trackStoreContents(bgCtx, haul, filepath.Base(f)); err != nil {
						log.Printf("Warning: failed to track contents for %s: %v", f, err)
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
		"cleared":   req.Clear,
		"haulId":    haul.ID,
	})
}

// CopyRequest represents the request to copy the store to a registry or directory
type CopyRequest struct {
	HaulID    int64  `json:"haulId,omitempty"`
	Target    string `json:"target"`
	Insecure  bool   `json:"insecure"`
	PlainHTTP bool   `json:"plainHttp"`
	Only      string `json:"only,omitempty"`
}

// RemoveRequest represents the request to remove artifacts from the store
type RemoveRequest struct {
	HaulID int64  `json:"haulId,omitempty"`
	Match  string `json:"match"`
	Force  bool   `json:"force"`
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

	haul, storeArgs, err := h.resolveHaul(r.Context(), req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
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

	// Scope to this haul's store.
	args = append(args, storeArgs...)

	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating copy job: %v", err)
		http.Error(w, "Failed to create copy job", http.StatusInternalServerError)
		return
	}
	h.tagJobHaul(r.Context(), job.ID, haul.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":   job.ID,
		"message": "Copy job started",
		"target":  req.Target,
		"haulId":  haul.ID,
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

	haul, storeArgs, err := h.resolveHaul(r.Context(), req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Build args for hauler store remove command
	args := []string{"store", "remove", req.Match}

	// Optional force flag to bypass confirmation
	if req.Force {
		args = append(args, "--force")
	}

	// Scope to this haul's store.
	args = append(args, storeArgs...)

	job, err := h.JobRunner.CreateJob(r.Context(), "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating remove job: %v", err)
		http.Error(w, "Failed to create remove job", http.StatusInternalServerError)
		return
	}
	h.tagJobHaul(r.Context(), job.ID, haul.ID)
	go h.rescanAfterJob(job.ID, haul)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId":   job.ID,
		"message": "Remove job started",
		"match":   req.Match,
		"force":   req.Force,
		"haulId":  haul.ID,
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

	ctx := r.Context()

	// Resolve which haul's store to inspect.
	haulID, _ := strconv.ParseInt(r.URL.Query().Get("haul"), 10, 64)
	haul, _, err := h.resolveHaul(ctx, haulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Build args for hauler store info command with JSON output, scoped to the haul.
	args := []string{"store", "info", "-o", "json", "--store", haul.StoreDir}

	// Run hauler store info command directly
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
	rows, err := db.QueryContext(ctx, `SELECT name, digest, source_haul FROM store_contents WHERE haul_id = ?`, haul.ID)
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

// clearStore removes a haul's store directory and recreates the OCI layout structure
func (h *Handler) clearStore(storeDir string) error {
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

// storeItem is a single artifact discovered in a haul's store index.
type storeItem struct {
	ContentType string
	Name        string
	Digest      string
}

// readStoreItems parses a haul store's index.json into a list of artifacts,
// deriving each item's name from its OCI annotations (the index manifests have
// no top-level name field). Returns an empty slice for a fresh/empty store.
func readStoreItems(storeDir string) ([]storeItem, error) {
	indexData, err := os.ReadFile(filepath.Join(storeDir, "index.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // fresh store, nothing tracked yet
		}
		return nil, fmt.Errorf("reading index.json: %w", err)
	}

	var index struct {
		Manifests []struct {
			Digest      string                 `json:"digest"`
			Annotations map[string]interface{} `json:"annotations"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(indexData, &index); err != nil {
		return nil, fmt.Errorf("parsing index.json: %w", err)
	}

	items := make([]storeItem, 0, len(index.Manifests))
	for _, m := range index.Manifests {
		// Prefer io.containerd.image.name (full reference) over the short ref name.
		var name string
		if m.Annotations != nil {
			if n, ok := m.Annotations["io.containerd.image.name"].(string); ok {
				name = n
			} else if n, ok := m.Annotations["org.opencontainers.image.ref.name"].(string); ok {
				name = n
			}
		}
		if name == "" {
			continue
		}

		contentType := "file"
		if strings.Contains(name, ":") {
			contentType = "image"
		} else if strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".tar.gz") {
			contentType = "chart"
		}
		items = append(items, storeItem{ContentType: contentType, Name: name, Digest: m.Digest})
	}
	return items, nil
}

// trackStoreContents records the artifacts currently in a haul's store. Existing
// rows are preserved (INSERT OR IGNORE) so provenance from prior loads is not
// clobbered by later direct adds; sourceArchive is recorded for newly seen items.
func (h *Handler) trackStoreContents(ctx context.Context, haul *hauls.Haul, sourceArchive string) error {
	items, err := readStoreItems(haul.StoreDir)
	if err != nil {
		return err
	}

	var source interface{}
	if sourceArchive != "" {
		source = sourceArchive
	}

	db := h.JobRunner.DB()
	for _, it := range items {
		if _, err := db.ExecContext(ctx, `
			INSERT OR IGNORE INTO store_contents (haul_id, content_type, name, digest, source_haul)
			VALUES (?, ?, ?, ?, ?)
		`, haul.ID, it.ContentType, it.Name, it.Digest, source); err != nil {
			log.Printf("Error inserting store content %s: %v", it.Name, err)
		}
	}
	log.Printf("Tracked %d items into haul %d (source=%q)", len(items), haul.ID, sourceArchive)
	return nil
}

// trackAfterJob waits for a store-modifying job to finish, then records the
// haul's current contents so summary counts and the contents view stay accurate.
func (h *Handler) trackAfterJob(jobID int64, haul *hauls.Haul) {
	ctx := context.Background()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		j, err := h.JobRunner.GetJob(ctx, jobID)
		if err != nil {
			return
		}
		switch j.Status {
		case jobrunner.StatusSucceeded:
			if err := h.trackStoreContents(ctx, haul, ""); err != nil {
				log.Printf("Warning: failed to track contents for haul %d: %v", haul.ID, err)
			}
			return
		case jobrunner.StatusFailed:
			return
		}
	}
}

// rescanAfterJob waits for a job (e.g. remove) to finish, then fully rebuilds the
// haul's tracked contents so deletions are reflected in the counts.
func (h *Handler) rescanAfterJob(jobID int64, haul *hauls.Haul) {
	ctx := context.Background()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		j, err := h.JobRunner.GetJob(ctx, jobID)
		if err != nil {
			return
		}
		switch j.Status {
		case jobrunner.StatusSucceeded:
			if _, err := h.rescanStore(ctx, haul); err != nil {
				log.Printf("Warning: failed to rescan haul %d: %v", haul.ID, err)
			}
			return
		case jobrunner.StatusFailed:
			return
		}
	}
}

// rescanStore rebuilds a haul's store_contents rows from scratch (used after
// removals and by the manual Rescan endpoint). Source provenance is reset.
func (h *Handler) rescanStore(ctx context.Context, haul *hauls.Haul) (int, error) {
	items, err := readStoreItems(haul.StoreDir)
	if err != nil {
		return 0, err
	}

	db := h.JobRunner.DB()
	if _, err := db.ExecContext(ctx, "DELETE FROM store_contents WHERE haul_id = ?", haul.ID); err != nil {
		return 0, fmt.Errorf("clearing store_contents: %w", err)
	}

	count := 0
	for _, it := range items {
		if _, err := db.ExecContext(ctx, `
			INSERT OR IGNORE INTO store_contents (haul_id, content_type, name, digest, source_haul, loaded_at)
			VALUES (?, ?, ?, ?, NULL, datetime('now'))
		`, haul.ID, it.ContentType, it.Name, it.Digest); err != nil {
			log.Printf("Error inserting store content %s: %v", it.Name, err)
		} else {
			count++
		}
	}
	log.Printf("Rescan complete for haul %d: tracked %d items", haul.ID, count)
	return count, nil
}

// Rescan handles POST /api/store/rescan
func (h *Handler) Rescan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	haulID, _ := strconv.ParseInt(r.URL.Query().Get("haul"), 10, 64)
	haul, _, err := h.resolveHaul(r.Context(), haulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
		return
	}

	count, err := h.rescanStore(r.Context(), haul)
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

// RegisterRoutes registers the store routes with the given mux. Operations are
// scoped to a haul via a "haulId" field in the request body (or "?haul=" query
// for reads); when omitted they fall back to the default haul.
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
	mux.HandleFunc("/api/store/import", h.Import)
}

// Import handles POST /api/store/import. It accepts a .tar.zst upload, saves it
// into the target haul's archives directory, and kicks off a load so the
// archive's contents land in that haul's isolated store.
//
// Form fields: file (the archive), haulId (target haul), clear ("true" to wipe
// the haul's store before loading).
func (h *Handler) Import(w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()
	haulID, _ := strconv.ParseInt(r.FormValue("haulId"), 10, 64)
	haul, storeArgs, err := h.resolveHaul(ctx, haulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
		return
	}
	clear := r.FormValue("clear") == "true"

	// Get the file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error getting file from form: %v", err)
		http.Error(w, "No file provided or error reading file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := header.Filename
	if !strings.HasSuffix(strings.ToLower(filename), ".tar.zst") {
		http.Error(w, "Only .tar.zst files are allowed", http.StatusBadRequest)
		return
	}
	if strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(haul.ArchivesDir(), 0755); err != nil {
		log.Printf("Error creating archives directory: %v", err)
		http.Error(w, "Failed to create archives directory", http.StatusInternalServerError)
		return
	}

	destinationPath := filepath.Join(haul.ArchivesDir(), filename)
	destFile, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("Error creating destination file: %v", err)
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer destFile.Close()

	written, err := io.Copy(destFile, file)
	if err != nil {
		log.Printf("Error copying file: %v", err)
		os.Remove(destinationPath)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	log.Printf("Imported archive into haul %d: %s (%d bytes)", haul.ID, filename, written)

	// Optionally clear the haul's store before loading.
	if clear {
		if err := h.clearStore(haul.StoreDir); err != nil {
			http.Error(w, "Failed to clear store: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := h.JobRunner.DB().ExecContext(ctx, `DELETE FROM store_contents WHERE haul_id = ?`, haul.ID); err != nil {
			log.Printf("Warning: failed to clear tracked contents for haul %d: %v", haul.ID, err)
		}
	}

	// Kick off a load of the freshly uploaded archive into the haul's store.
	args := []string{"store", "load", "-f", destinationPath}
	args = append(args, storeArgs...)
	job, err := h.JobRunner.CreateJob(ctx, "hauler", args, nil)
	if err != nil {
		log.Printf("Error creating load job: %v", err)
		http.Error(w, "Failed to create load job", http.StatusInternalServerError)
		return
	}
	h.tagJobHaul(ctx, job.ID, haul.ID)

	jobID := job.ID
	go func() {
		bgCtx := context.Background()
		for {
			time.Sleep(500 * time.Millisecond)
			j, err := h.JobRunner.GetJob(bgCtx, jobID)
			if err != nil {
				return
			}
			if j.Status == jobrunner.StatusSucceeded {
				if err := h.trackStoreContents(bgCtx, haul, filename); err != nil {
					log.Printf("Warning: failed to track contents for %s: %v", filename, err)
				}
				return
			}
			if j.Status == jobrunner.StatusFailed {
				return
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Archive imported, load started",
		"filename": filename,
		"size":     written,
		"jobId":    job.ID,
		"haulId":   haul.ID,
	})
}
