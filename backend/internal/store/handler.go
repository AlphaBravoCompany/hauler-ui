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

// RegisterRoutes registers the store routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/store/add-image", h.AddImage)
}
