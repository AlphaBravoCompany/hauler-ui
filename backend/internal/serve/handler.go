package serve

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
	"sync"
	"syscall"
	"time"

	_ "modernc.org/sqlite"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
	"github.com/hauler-ui/hauler-ui/backend/internal/hauls"
)

// Handler handles HTTP requests for serve operations
type Handler struct {
	cfg         *config.Config
	db          *sql.DB
	hauls       *hauls.Service
	processes   map[int]*managedProcess
	certManager *CertManager
	mu          sync.RWMutex
}

type managedProcess struct {
	Cmd       *exec.Cmd
	Process   *os.Process
	StartedAt time.Time
	Logs      []string
	LogMu     sync.Mutex
}

// NewHandler creates a new serve handler
func NewHandler(cfg *config.Config, db *sql.DB, haulSvc *hauls.Service) *Handler {
	return &Handler{
		cfg:         cfg,
		db:          db,
		hauls:       haulSvc,
		processes:   make(map[int]*managedProcess),
		certManager: NewCertManager(cfg.DataDir),
	}
}

// resolveHaul returns the haul to serve, defaulting to the default haul.
func (h *Handler) resolveHaul(ctx context.Context, haulID int64) (*hauls.Haul, error) {
	if haulID > 0 {
		return h.hauls.Get(ctx, haulID)
	}
	return h.hauls.EnsureDefault(ctx)
}

// portInUse reports whether a serve process is already running on the port.
func (h *Handler) portInUse(port int) bool {
	var count int
	_ = h.db.QueryRow(`SELECT COUNT(1) FROM serve_processes WHERE port = ? AND status = 'running'`, port).Scan(&count)
	return count > 0
}

// ServeRegistryRequest represents the request to start a registry serve
type ServeRegistryRequest struct {
	HaulID     int64  `json:"haulId,omitempty"`
	Port       int    `json:"port,omitempty"`
	Readonly   bool   `json:"readonly,omitempty"`
	TLSCert    string `json:"tlsCert,omitempty"`
	TLSKey     string `json:"tlsKey,omitempty"`
	AutoTLS    bool   `json:"autoTls,omitempty"`
	Directory  string `json:"directory,omitempty"`
	ConfigFile string `json:"configFile,omitempty"`
}

// ServeFileserverRequest represents the request to start a fileserver serve
type ServeFileserverRequest struct {
	HaulID    int64  `json:"haulId,omitempty"`
	Port      int    `json:"port,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
	TLSCert   string `json:"tlsCert,omitempty"`
	TLSKey    string `json:"tlsKey,omitempty"`
	AutoTLS   bool   `json:"autoTls,omitempty"`
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

	// Resolve which haul's store to serve.
	haul, err := h.resolveHaul(r.Context(), req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Set default port
	port := req.Port
	if port == 0 {
		port = 5000
	}

	// Prevent port collisions across concurrently served hauls.
	if h.portInUse(port) {
		http.Error(w, fmt.Sprintf("Port %d is already in use by another serve process", port), http.StatusConflict)
		return
	}

	// Build args for hauler store serve registry command, scoped to the haul.
	args := []string{"store", "serve", "registry", "--port", strconv.Itoa(port), "--store", haul.StoreDir}

	// Give each haul its own registry backend directory so multiple registries
	// can run side by side without clobbering each other.
	if req.Directory == "" {
		req.Directory = filepath.Join(filepath.Dir(haul.StoreDir), "registry")
	}

	// Readonly flag (hauler default is true)
	if req.Readonly {
		args = append(args, "--readonly")
	} else {
		args = append(args, "--readonly=false")
	}

	// Handle TLS configuration
	if req.AutoTLS {
		serveType := "registry"
		certPath, keyPath, err := h.certManager.GetOrGenerateCert(serveType)
		if err != nil {
			log.Printf("Error generating TLS certificate: %v", err)
			http.Error(w, fmt.Sprintf("Failed to generate certificate: %v", err), http.StatusInternalServerError)
			return
		}
		args = append(args, "--tls-cert", certPath, "--tls-key", keyPath)
	} else {
		// Optional TLS cert
		if req.TLSCert != "" {
			args = append(args, "--tls-cert", req.TLSCert)
		}

		// Optional TLS key
		if req.TLSKey != "" {
			args = append(args, "--tls-key", req.TLSKey)
		}
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
		INSERT INTO serve_processes (serve_type, pid, port, args, status, haul_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "registry", pid, port, string(argsJSON), "running", haul.ID)
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
		"haulId":    haul.ID,
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

	processes, err := h.queryProcesses("registry", r.URL.Query().Get("haul"))
	if err != nil {
		http.Error(w, "Query error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(processes)
}

// queryProcesses returns serve processes of a given type, optionally filtered by
// haul id, including the haul each process is bound to.
func (h *Handler) queryProcesses(serveType, haulFilter string) ([]map[string]interface{}, error) {
	query := `
		SELECT id, serve_type, pid, port, args, status, started_at, stopped_at, exit_reason, haul_id
		FROM serve_processes
		WHERE serve_type = ?`
	queryArgs := []interface{}{serveType}
	if haulFilter != "" {
		if haulID, err := strconv.ParseInt(haulFilter, 10, 64); err == nil {
			query += ` AND haul_id = ?`
			queryArgs = append(queryArgs, haulID)
		}
	}
	query += ` ORDER BY started_at DESC`

	rows, err := h.db.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	processes := []map[string]interface{}{}
	for rows.Next() {
		var id, pid, port int
		var st, argsJSON, status string
		var startedAt, stoppedAt, exitReason sql.NullString
		var haulID sql.NullInt64

		if err := rows.Scan(&id, &st, &pid, &port, &argsJSON, &status, &startedAt, &stoppedAt, &exitReason, &haulID); err != nil {
			continue
		}

		proc := map[string]interface{}{
			"id":         id,
			"serveType":  st,
			"pid":        pid,
			"port":       port,
			"status":     status,
			"startedAt":  startedAt.String,
			"stoppedAt":  stoppedAt.String,
			"exitReason": exitReason.String,
		}
		if haulID.Valid {
			proc["haulId"] = haulID.Int64
		}
		if argsJSON != "" {
			var args map[string]interface{}
			_ = json.Unmarshal([]byte(argsJSON), &args)
			proc["args"] = args
		}
		processes = append(processes, proc)
	}
	return processes, rows.Err()
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

	// Resolve which haul's store to serve.
	haul, err := h.resolveHaul(r.Context(), req.HaulID)
	if err != nil {
		http.Error(w, "Failed to resolve haul: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Set default port
	port := req.Port
	if port == 0 {
		port = 8080
	}

	// Prevent port collisions across concurrently served hauls.
	if h.portInUse(port) {
		http.Error(w, fmt.Sprintf("Port %d is already in use by another serve process", port), http.StatusConflict)
		return
	}

	// Build args for hauler store serve fileserver command, scoped to the haul.
	args := []string{"store", "serve", "fileserver", "--port", strconv.Itoa(port), "--store", haul.StoreDir}

	// Optional timeout
	if req.Timeout > 0 {
		args = append(args, "--timeout", strconv.Itoa(req.Timeout))
	}

	// Handle TLS configuration
	if req.AutoTLS {
		serveType := "fileserver"
		certPath, keyPath, err := h.certManager.GetOrGenerateCert(serveType)
		if err != nil {
			log.Printf("Error generating TLS certificate: %v", err)
			http.Error(w, fmt.Sprintf("Failed to generate certificate: %v", err), http.StatusInternalServerError)
			return
		}
		args = append(args, "--tls-cert", certPath, "--tls-key", keyPath)
	} else {
		// Optional TLS cert
		if req.TLSCert != "" {
			args = append(args, "--tls-cert", req.TLSCert)
		}

		// Optional TLS key
		if req.TLSKey != "" {
			args = append(args, "--tls-key", req.TLSKey)
		}
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
		INSERT INTO serve_processes (serve_type, pid, port, args, status, haul_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "fileserver", pid, port, string(argsJSON), "running", haul.ID)
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
		"haulId":    haul.ID,
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

	processes, err := h.queryProcesses("fileserver", r.URL.Query().Get("haul"))
	if err != nil {
		http.Error(w, "Query error", http.StatusInternalServerError)
		return
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
