package serve

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "modernc.org/sqlite"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
)

// Handler handles HTTP requests for serve operations
type Handler struct {
	cfg       *config.Config
	db        *sql.DB
	processes map[int]*managedProcess
	mu        sync.RWMutex
}

type managedProcess struct {
	Cmd       *exec.Cmd
	Process   *os.Process
	StartedAt time.Time
	Logs      []string
	LogMu     sync.Mutex
}

// NewHandler creates a new serve handler
func NewHandler(cfg *config.Config, db *sql.DB) *Handler {
	return &Handler{
		cfg:       cfg,
		db:        db,
		processes: make(map[int]*managedProcess),
	}
}

// ServeRegistryRequest represents the request to start a registry serve
type ServeRegistryRequest struct {
	Port       int    `json:"port,omitempty"`
	Readonly   bool   `json:"readonly,omitempty"`
	TLSCert    string `json:"tlsCert,omitempty"`
	TLSKey     string `json:"tlsKey,omitempty"`
	Directory  string `json:"directory,omitempty"`
	ConfigFile string `json:"configFile,omitempty"`
}

// ServeFileserverRequest represents the request to start a fileserver serve
type ServeFileserverRequest struct {
	Port      int    `json:"port,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
	TLSCert   string `json:"tlsCert,omitempty"`
	TLSKey    string `json:"tlsKey,omitempty"`
	Directory string `json:"directory,omitempty"`
}

// ServeRegistry handles POST /api/serve/registry
func (h *Handler) ServeRegistry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ServeRegistryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Set default port
	port := req.Port
	if port == 0 {
		port = 5000
	}

	// Build args for hauler store serve registry command
	args := []string{"store", "serve", "registry", "--port", strconv.Itoa(port)}

	// Readonly flag (default true)
	if !req.Readonly {
		args = append(args, "--read-only=false")
	} else {
		args = append(args, "--read-only=true")
	}

	// Optional TLS cert
	if req.TLSCert != "" {
		args = append(args, "--tls-cert", req.TLSCert)
	}

	// Optional TLS key
	if req.TLSKey != "" {
		args = append(args, "--tls-key", req.TLSKey)
	}

	// Optional directory
	if req.Directory != "" {
		args = append(args, "--directory", req.Directory)
	}

	// Optional config file
	if req.ConfigFile != "" {
		args = append(args, "--config", req.ConfigFile)
	}

	// Start the process
	cmd := exec.Command("hauler", args...)
	cmd.Dir = h.cfg.DataDir

	// Capture stdout and stderr for log streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Error creating stdout pipe: %v", err)
		http.Error(w, "Failed to create stdout pipe", http.StatusInternalServerError)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("Error creating stderr pipe: %v", err)
		http.Error(w, "Failed to create stderr pipe", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Error starting registry serve: %v", err)
		http.Error(w, fmt.Sprintf("Failed to start registry serve: %v", err), http.StatusInternalServerError)
		return
	}

	pid := cmd.Process.Pid

	// Track the managed process
	managedProc := &managedProcess{
		Cmd:       cmd,
		Process:   cmd.Process,
		StartedAt: time.Now(),
		Logs:      []string{},
	}

	h.mu.Lock()
	h.processes[pid] = managedProc
	h.mu.Unlock()

	// Start a goroutine to monitor the process and capture logs
	go h.monitorProcess(pid, cmd, stdout, stderr)

	// Store in database
	argsJSON, _ := json.Marshal(req)
	_, err = h.db.Exec(`
		INSERT INTO serve_processes (serve_type, pid, port, args, status)
		VALUES (?, ?, ?, ?, ?)
	`, "registry", pid, port, string(argsJSON), "running")
	if err != nil {
		log.Printf("Error storing serve process in database: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"pid":       pid,
		"port":      port,
		"status":    "running",
		"startedAt": managedProc.StartedAt.Format(time.RFC3339),
		"message":   "Registry serve started",
	})
}

// monitorProcess monitors a running process and captures its output
func (h *Handler) monitorProcess(pid int, cmd *exec.Cmd, stdout, stderr io.ReadCloser) {
	// Close pipes when done
	defer stdout.Close()
	defer stderr.Close()

	// Wait for process to complete
	err := cmd.Wait()

	// Capture final status
	h.mu.Lock()
	managedProc, exists := h.processes[pid]
	if exists {
		managedProc.LogMu.Lock()
		if err != nil {
			managedProc.Logs = append(managedProc.Logs, fmt.Sprintf("Process exited: %v", err))
		} else {
			managedProc.Logs = append(managedProc.Logs, "Process exited cleanly")
		}
		managedProc.LogMu.Unlock()
		delete(h.processes, pid)
	}
	h.mu.Unlock()

	// Update database
	exitReason := ""
	if err != nil {
		exitReason = err.Error()
	}
	_, dbErr := h.db.Exec(`
		UPDATE serve_processes
		SET status = ?, stopped_at = CURRENT_TIMESTAMP, exit_reason = ?
		WHERE pid = ?
	`, "stopped", exitReason, pid)
	if dbErr != nil {
		log.Printf("Error updating serve process in database: %v", dbErr)
	}
}

// StopRegistry handles DELETE /api/serve/registry/:pid
func (h *Handler) StopRegistry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract PID from path
	// Path format: /api/serve/registry/:pid
	prefix := "/api/serve/registry/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	pidStr := r.URL.Path[len(prefix):]
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		http.Error(w, "Invalid PID", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	managedProc, exists := h.processes[pid]
	h.mu.RUnlock()

	if !exists {
		// Check database for historical record
		var status string
		row := h.db.QueryRow("SELECT status FROM serve_processes WHERE pid = ?", pid)
		_ = row.Scan(&status)
		if status == "stopped" {
			http.Error(w, "Process already stopped", http.StatusGone)
			return
		}
		http.Error(w, "Process not found", http.StatusNotFound)
		return
	}

	// Send SIGTERM for graceful shutdown
	if err := managedProc.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("Error sending SIGTERM to process %d: %v", pid, err)
		http.Error(w, fmt.Sprintf("Failed to stop process: %v", err), http.StatusInternalServerError)
		return
	}

	// Update database immediately
	_, _ = h.db.Exec(`
		UPDATE serve_processes
		SET status = ?, stopped_at = CURRENT_TIMESTAMP
		WHERE pid = ?
	`, "stopped", pid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"pid":     pid,
		"status":  "stopped",
		"message": "Registry serve stopped",
	})
}

// GetRegistryStatus handles GET /api/serve/registry/:pid
func (h *Handler) GetRegistryStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract PID from path
	prefix := "/api/serve/registry/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	pidStr := r.URL.Path[len(prefix):]
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		http.Error(w, "Invalid PID", http.StatusBadRequest)
		return
	}

	// Check in-memory map first
	h.mu.RLock()
	managedProc, inMemory := h.processes[pid]
	if inMemory {
		managedProc.LogMu.Lock()
		logs := make([]string, len(managedProc.Logs))
		copy(logs, managedProc.Logs)
		managedProc.LogMu.Unlock()
		h.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"pid":       pid,
			"status":    "running",
			"startedAt": managedProc.StartedAt.Format(time.RFC3339),
			"logs":      logs,
		})
		return
	}
	h.mu.RUnlock()

	// Check database for historical record
	var serveType string
	var port int
	var argsJSON string
	var status string
	var startedAt, stoppedAt sql.NullString
	var exitReason sql.NullString

	row := h.db.QueryRow(`
		SELECT serve_type, port, args, status, started_at, stopped_at, exit_reason
		FROM serve_processes
		WHERE pid = ?
		ORDER BY started_at DESC
		LIMIT 1
	`, pid)

	err = row.Scan(&serveType, &port, &argsJSON, &status, &startedAt, &stoppedAt, &exitReason)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Process not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	response := map[string]interface{}{
		"pid":        pid,
		"serveType":  serveType,
		"port":       port,
		"status":     status,
		"startedAt":  startedAt.String,
		"stoppedAt":  stoppedAt.String,
		"exitReason": exitReason.String,
	}

	if argsJSON != "" {
		var args map[string]interface{}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		response["args"] = args
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// ListRegistryProcesses handles GET /api/serve/registry
func (h *Handler) ListRegistryProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := h.db.Query(`
		SELECT id, serve_type, pid, port, args, status, started_at, stopped_at, exit_reason
		FROM serve_processes
		WHERE serve_type = 'registry'
		ORDER BY started_at DESC
	`)
	if err != nil {
		http.Error(w, "Query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	processes := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var serveType string
		var pid int
		var port int
		var argsJSON string
		var status string
		var startedAt, stoppedAt sql.NullString
		var exitReason sql.NullString

		if err := rows.Scan(&id, &serveType, &pid, &port, &argsJSON, &status, &startedAt, &stoppedAt, &exitReason); err != nil {
			continue
		}

		proc := map[string]interface{}{
			"id":         id,
			"serveType":  serveType,
			"pid":        pid,
			"port":       port,
			"status":     status,
			"startedAt":  startedAt.String,
			"stoppedAt":  stoppedAt.String,
			"exitReason": exitReason.String,
		}

		if argsJSON != "" {
			var args map[string]interface{}
			_ = json.Unmarshal([]byte(argsJSON), &args)
			proc["args"] = args
		}

		processes = append(processes, proc)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(processes)
}

// ServeFileserver handles POST /api/serve/fileserver
func (h *Handler) ServeFileserver(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ServeFileserverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Set default port
	port := req.Port
	if port == 0 {
		port = 8080
	}

	// Build args for hauler store serve fileserver command
	args := []string{"store", "serve", "fileserver", "--port", strconv.Itoa(port)}

	// Optional timeout
	if req.Timeout > 0 {
		args = append(args, "--timeout", strconv.Itoa(req.Timeout))
	}

	// Optional TLS cert
	if req.TLSCert != "" {
		args = append(args, "--tls-cert", req.TLSCert)
	}

	// Optional TLS key
	if req.TLSKey != "" {
		args = append(args, "--tls-key", req.TLSKey)
	}

	// Optional directory
	if req.Directory != "" {
		args = append(args, "--directory", req.Directory)
	}

	// Start the process
	cmd := exec.Command("hauler", args...)
	cmd.Dir = h.cfg.DataDir

	// Capture stdout and stderr for log streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Error creating stdout pipe: %v", err)
		http.Error(w, "Failed to create stdout pipe", http.StatusInternalServerError)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("Error creating stderr pipe: %v", err)
		http.Error(w, "Failed to create stderr pipe", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Error starting fileserver serve: %v", err)
		http.Error(w, fmt.Sprintf("Failed to start fileserver serve: %v", err), http.StatusInternalServerError)
		return
	}

	pid := cmd.Process.Pid

	// Track the managed process
	managedProc := &managedProcess{
		Cmd:       cmd,
		Process:   cmd.Process,
		StartedAt: time.Now(),
		Logs:      []string{},
	}

	h.mu.Lock()
	h.processes[pid] = managedProc
	h.mu.Unlock()

	// Start a goroutine to monitor the process and capture logs
	go h.monitorProcess(pid, cmd, stdout, stderr)

	// Store in database
	argsJSON, _ := json.Marshal(req)
	_, err = h.db.Exec(`
		INSERT INTO serve_processes (serve_type, pid, port, args, status)
		VALUES (?, ?, ?, ?, ?)
	`, "fileserver", pid, port, string(argsJSON), "running")
	if err != nil {
		log.Printf("Error storing serve process in database: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"pid":       pid,
		"port":      port,
		"status":    "running",
		"startedAt": managedProc.StartedAt.Format(time.RFC3339),
		"message":   "Fileserver serve started",
	})
}

// StopFileserver handles DELETE /api/serve/fileserver/:pid
func (h *Handler) StopFileserver(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract PID from path
	// Path format: /api/serve/fileserver/:pid
	prefix := "/api/serve/fileserver/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	pidStr := r.URL.Path[len(prefix):]
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		http.Error(w, "Invalid PID", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	managedProc, exists := h.processes[pid]
	h.mu.RUnlock()

	if !exists {
		// Check database for historical record
		var status string
		row := h.db.QueryRow("SELECT status FROM serve_processes WHERE pid = ?", pid)
		_ = row.Scan(&status)
		if status == "stopped" {
			http.Error(w, "Process already stopped", http.StatusGone)
			return
		}
		http.Error(w, "Process not found", http.StatusNotFound)
		return
	}

	// Send SIGTERM for graceful shutdown
	if err := managedProc.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("Error sending SIGTERM to process %d: %v", pid, err)
		http.Error(w, fmt.Sprintf("Failed to stop process: %v", err), http.StatusInternalServerError)
		return
	}

	// Update database immediately
	_, _ = h.db.Exec(`
		UPDATE serve_processes
		SET status = ?, stopped_at = CURRENT_TIMESTAMP
		WHERE pid = ?
	`, "stopped", pid)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"pid":     pid,
		"status":  "stopped",
		"message": "Fileserver serve stopped",
	})
}

// GetFileserverStatus handles GET /api/serve/fileserver/:pid
func (h *Handler) GetFileserverStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract PID from path
	prefix := "/api/serve/fileserver/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	pidStr := r.URL.Path[len(prefix):]
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		http.Error(w, "Invalid PID", http.StatusBadRequest)
		return
	}

	// Check in-memory map first
	h.mu.RLock()
	managedProc, inMemory := h.processes[pid]
	if inMemory {
		managedProc.LogMu.Lock()
		logs := make([]string, len(managedProc.Logs))
		copy(logs, managedProc.Logs)
		managedProc.LogMu.Unlock()
		h.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"pid":       pid,
			"status":    "running",
			"startedAt": managedProc.StartedAt.Format(time.RFC3339),
			"logs":      logs,
		})
		return
	}
	h.mu.RUnlock()

	// Check database for historical record
	var serveType string
	var port int
	var argsJSON string
	var status string
	var startedAt, stoppedAt sql.NullString
	var exitReason sql.NullString

	row := h.db.QueryRow(`
		SELECT serve_type, port, args, status, started_at, stopped_at, exit_reason
		FROM serve_processes
		WHERE pid = ?
		ORDER BY started_at DESC
		LIMIT 1
	`, pid)

	err = row.Scan(&serveType, &port, &argsJSON, &status, &startedAt, &stoppedAt, &exitReason)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Process not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	response := map[string]interface{}{
		"pid":        pid,
		"serveType":  serveType,
		"port":       port,
		"status":     status,
		"startedAt":  startedAt.String,
		"stoppedAt":  stoppedAt.String,
		"exitReason": exitReason.String,
	}

	if argsJSON != "" {
		var args map[string]interface{}
		_ = json.Unmarshal([]byte(argsJSON), &args)
		response["args"] = args
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// ListFileserverProcesses handles GET /api/serve/fileserver
func (h *Handler) ListFileserverProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := h.db.Query(`
		SELECT id, serve_type, pid, port, args, status, started_at, stopped_at, exit_reason
		FROM serve_processes
		WHERE serve_type = 'fileserver'
		ORDER BY started_at DESC
	`)
	if err != nil {
		http.Error(w, "Query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	processes := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var serveType string
		var pid int
		var port int
		var argsJSON string
		var status string
		var startedAt, stoppedAt sql.NullString
		var exitReason sql.NullString

		if err := rows.Scan(&id, &serveType, &pid, &port, &argsJSON, &status, &startedAt, &stoppedAt, &exitReason); err != nil {
			continue
		}

		proc := map[string]interface{}{
			"id":         id,
			"serveType":  serveType,
			"pid":        pid,
			"port":       port,
			"status":     status,
			"startedAt":  startedAt.String,
			"stoppedAt":  stoppedAt.String,
			"exitReason": exitReason.String,
		}

		if argsJSON != "" {
			var args map[string]interface{}
			_ = json.Unmarshal([]byte(argsJSON), &args)
			proc["args"] = args
		}

		processes = append(processes, proc)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(processes)
}

// RegisterRoutes registers the serve routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/serve/registry", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.ServeRegistry(w, r)
		case http.MethodGet:
			h.ListRegistryProcesses(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/serve/registry/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetRegistryStatus(w, r)
		case http.MethodDelete:
			h.StopRegistry(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/serve/fileserver", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.ServeFileserver(w, r)
		case http.MethodGet:
			h.ListFileserverProcesses(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/serve/fileserver/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.GetFileserverStatus(w, r)
		case http.MethodDelete:
			h.StopFileserver(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
