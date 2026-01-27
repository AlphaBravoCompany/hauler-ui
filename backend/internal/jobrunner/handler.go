package jobrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
)

// Handler handles HTTP requests for job management
type Handler struct {
	runner    *Runner
	cfg       *config.Config
	mu        sync.RWMutex
	clients   map[int64][]chan struct{} // map jobID to list of broadcast channels
}

// NewHandler creates a new job handler
func NewHandler(runner *Runner, cfg *config.Config) *Handler {
	return &Handler{
		runner:  runner,
		cfg:     cfg,
		clients: make(map[int64][]chan struct{}),
	}
}

// CreateJobRequest represents the request to create a new job
type CreateJobRequest struct {
	Command      string            `json:"command"`
	Args         []string          `json:"args"`
	EnvOverrides map[string]string `json:"envOverrides"`
}

// CreateJob handles POST /api/jobs
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Command == "" {
		http.Error(w, "command is required", http.StatusBadRequest)
		return
	}

	job, err := h.runner.CreateJob(r.Context(), req.Command, req.Args, req.EnvOverrides)
	if err != nil {
		log.Printf("Error creating job: %v", err)
		http.Error(w, "Failed to create job", http.StatusInternalServerError)
		return
	}

	// Start the job in background
	go func() {
		if err := h.runner.Start(context.Background(), job.ID); err != nil {
			log.Printf("Error starting job %d: %v", job.ID, err)
			h.notifyClients(job.ID)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(job)
}

// GetJob handles GET /api/jobs/:id
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID, err := parseID(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	job, err := h.runner.GetJob(r.Context(), jobID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		log.Printf("Error getting job: %v", err)
		http.Error(w, "Failed to get job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}

// ListJobs handles GET /api/jobs
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var status *JobStatus
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		s := JobStatus(statusStr)
		status = &s
	}

	jobs, err := h.runner.ListJobs(r.Context(), status)
	if err != nil {
		log.Printf("Error listing jobs: %v", err)
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jobs)
}

// GetJobLogs handles GET /api/jobs/:id/logs
func (h *Handler) GetJobLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID, err := parseID(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	var since *time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if ts, err := time.Parse(time.RFC3339Nano, sinceStr); err == nil {
			since = &ts
		}
	}

	logs, err := h.runner.GetLogs(r.Context(), jobID, since)
	if err != nil {
		log.Printf("Error getting logs: %v", err)
		http.Error(w, "Failed to get logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(logs)
}

// StreamJobLogs handles GET /api/jobs/:id/stream - SSE endpoint for streaming logs
func (h *Handler) StreamJobLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID, err := parseID(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	// Check if job exists
	job, err := h.runner.GetJob(r.Context(), jobID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to get job", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Flush headers
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	// Register client for notifications
	notifyCh := make(chan struct{}, 1)
	h.registerClient(jobID, notifyCh)
	defer h.unregisterClient(jobID, notifyCh)

	// Create a context for this connection
	ctx := r.Context()

	var lastTimestamp time.Time

	// Send initial state
	if err := h.sendJobState(w, job); err != nil {
		return
	}
	flusher.Flush()

	// Stream logs
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-notifyCh:
			// Get updated job state
			updatedJob, err := h.runner.GetJob(ctx, jobID)
			if err != nil {
				return
			}

			// Get new logs
			logs, err := h.runner.GetLogs(ctx, jobID, &lastTimestamp)
			if err != nil {
				log.Printf("Error getting logs: %v", err)
				continue
			}

			// Send log lines
			for _, logEntry := range logs {
				if err := h.sendSSE(w, "log", map[string]interface{}{
					"stream":    logEntry.Stream,
					"content":   logEntry.Content,
					"timestamp": logEntry.Timestamp.Format(time.RFC3339Nano),
				}); err != nil {
					return
				}
				if logEntry.Timestamp.After(lastTimestamp) {
					lastTimestamp = logEntry.Timestamp
				}
			}

			// Send updated job state
			job = updatedJob
			if err := h.sendJobState(w, job); err != nil {
				return
			}
			flusher.Flush()

			// Exit if job is complete
			if job.Status == StatusSucceeded || job.Status == StatusFailed {
				// Send final completion event
				_ = h.sendSSE(w, "complete", job)
				flusher.Flush()
				return
			}

		case <-ticker.C:
			// Poll for any logs we might have missed
			logs, err := h.runner.GetLogs(ctx, jobID, &lastTimestamp)
			if err != nil {
				continue
			}

			for _, logEntry := range logs {
				if err := h.sendSSE(w, "log", map[string]interface{}{
					"stream":    logEntry.Stream,
					"content":   logEntry.Content,
					"timestamp": logEntry.Timestamp.Format(time.RFC3339Nano),
				}); err != nil {
					return
				}
				if logEntry.Timestamp.After(lastTimestamp) {
					lastTimestamp = logEntry.Timestamp
				}
			}
			flusher.Flush()
		}
	}
}

// sendJobState sends the job state via SSE
func (h *Handler) sendJobState(w http.ResponseWriter, job *Job) error {
	return h.sendSSE(w, "state", job)
}

// sendSSE sends a server-sent event
func (h *Handler) sendSSE(w http.ResponseWriter, event string, data interface{}) error {
	var jsonData []byte
	var err error

	if s, ok := data.(fmt.Stringer); ok {
		jsonData, err = json.Marshal(s.String())
	} else {
		jsonData, err = json.Marshal(data)
	}
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData); err != nil {
		return err
	}
	return nil
}

// registerClient adds a client channel for job notifications
func (h *Handler) registerClient(jobID int64, ch chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[jobID] = append(h.clients[jobID], ch)
}

// unregisterClient removes a client channel from job notifications
func (h *Handler) unregisterClient(jobID int64, ch chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.clients[jobID]
	for i, c := range clients {
		if c == ch {
			h.clients[jobID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(h.clients[jobID]) == 0 {
		delete(h.clients, jobID)
	}
}

// notifyClients notifies all clients listening for a job
func (h *Handler) notifyClients(jobID int64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.clients[jobID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// parseID extracts the job ID from the URL path
// Expects path like /api/jobs/123 or /api/jobs/123/logs or /api/jobs/123/stream
func parseID(path string) (int64, error) {
	// Remove /api/jobs/ prefix
	prefix := "/api/jobs/"
	if len(path) <= len(prefix) {
		return 0, fmt.Errorf("invalid path format")
	}

	rest := path[len(prefix):]
	// Find next slash to get just the ID
	for i, c := range rest {
		if c == '/' {
			rest = rest[:i]
			break
		}
	}

	return strconv.ParseInt(rest, 10, 64)
}
