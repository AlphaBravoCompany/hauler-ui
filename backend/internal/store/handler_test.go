package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
	"github.com/hauler-ui/hauler-ui/backend/internal/jobrunner"
)

func setupTestHandler(t *testing.T) (*Handler, *sql.DB) {
	t.Helper()

	f, err := os.CreateTemp("", "testdb-*.db")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer f.Close()
	name := f.Name()
	t.Cleanup(func() { os.Remove(name) })

	db, err := sql.Open("sqlite", name)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create schema
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			command TEXT NOT NULL,
			args TEXT,
			env_overrides TEXT,
			status TEXT NOT NULL DEFAULT 'queued',
			exit_code INTEGER,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			result TEXT
		);

		CREATE TABLE IF NOT EXISTS job_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER NOT NULL,
			stream TEXT NOT NULL,
			content TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_job_logs_job_id ON job_logs(job_id, timestamp);
	`)
	if err != nil {
		t.Fatalf("creating schema: %v", err)
	}

	cfg := &config.Config{
		HaulerTempDir: os.TempDir(),
		DataDir:       os.TempDir(),
	}

	runner := jobrunner.New(db)
	handler := NewHandler(runner, cfg)

	return handler, db
}

func TestCopyHandler_InvalidMethod(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/store/copy", nil)
	w := httptest.NewRecorder()

	handler.Copy(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestCopyHandler_MissingTarget(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := CopyRequest{}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest(http.MethodPost, "/api/store/copy", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Copy(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCopyHandler_InvalidTargetFormat(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := CopyRequest{Target: "invalid-target"}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest(http.MethodPost, "/api/store/copy", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Copy(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	respBody := w.Body.String()
	if !strings.Contains(respBody, "must start with registry:// or dir://") {
		t.Errorf("expected error message about target format, got: %s", respBody)
	}
}

func TestCopyHandler_ValidRegistryTarget(t *testing.T) {
	handler, db := setupTestHandler(t)

	req := CopyRequest{
		Target:    "registry://docker.io/my-org",
		Insecure:  true,
		PlainHTTP: false,
		Only:      "sig",
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest(http.MethodPost, "/api/store/copy", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Copy(w, r)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	jobID, ok := resp["jobId"].(float64)
	if !ok || jobID == 0 {
		t.Error("expected non-zero jobId in response")
	}

	// Verify job was created with correct args
	ctx := context.Background()
	job, err := handler.JobRunner.GetJob(ctx, int64(jobID))
	if err != nil {
		t.Fatalf("getting job: %v", err)
	}

	if job.Command != "hauler" {
		t.Errorf("expected command 'hauler', got %q", job.Command)
	}

	expectedArgs := []string{"store", "copy", "registry://docker.io/my-org", "--insecure", "--only", "sig"}
	if len(job.Args) != len(expectedArgs) {
		t.Errorf("expected %d args, got %d", len(expectedArgs), len(job.Args))
	} else {
		for i, arg := range expectedArgs {
			if job.Args[i] != arg {
				t.Errorf("arg %d: expected %q, got %q", i, arg, job.Args[i])
			}
		}
	}

	// Clean up job
	db.Exec("DELETE FROM jobs WHERE id = ?", job.ID)
}

func TestCopyHandler_ValidDirTarget(t *testing.T) {
	handler, db := setupTestHandler(t)

	req := CopyRequest{
		Target: "dir:///data/export",
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest(http.MethodPost, "/api/store/copy", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Copy(w, r)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	jobID, ok := resp["jobId"].(float64)
	if !ok || jobID == 0 {
		t.Error("expected non-zero jobId in response")
	}

	// Verify job was created with correct args (no insecure/plain-http for dir)
	ctx := context.Background()
	job, err := handler.JobRunner.GetJob(ctx, int64(jobID))
	if err != nil {
		t.Fatalf("getting job: %v", err)
	}

	expectedArgs := []string{"store", "copy", "dir:///data/export"}
	if len(job.Args) != len(expectedArgs) {
		t.Errorf("expected %d args, got %d", len(expectedArgs), len(job.Args))
	} else {
		for i, arg := range expectedArgs {
			if job.Args[i] != arg {
				t.Errorf("arg %d: expected %q, got %q", i, arg, job.Args[i])
			}
		}
	}

	// Clean up job
	db.Exec("DELETE FROM jobs WHERE id = ?", job.ID)
}

func TestCopyHandler_PlainHTTPFlag(t *testing.T) {
	handler, db := setupTestHandler(t)

	req := CopyRequest{
		Target:    "registry://localhost:5000/my-repo",
		PlainHTTP: true,
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest(http.MethodPost, "/api/store/copy", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Copy(w, r)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	jobID, _ := resp["jobId"].(float64)

	// Verify --plain-http flag was added
	ctx := context.Background()
	job, err := handler.JobRunner.GetJob(ctx, int64(jobID))
	if err != nil {
		t.Fatalf("getting job: %v", err)
	}

	hasPlainHTTP := false
	for _, arg := range job.Args {
		if arg == "--plain-http" {
			hasPlainHTTP = true
			break
		}
	}
	if !hasPlainHTTP {
		t.Error("expected --plain-http flag in args")
	}

	// Clean up job
	db.Exec("DELETE FROM jobs WHERE id = ?", job.ID)
}

func TestCopyHandler_OnlyAttestations(t *testing.T) {
	handler, db := setupTestHandler(t)

	req := CopyRequest{
		Target: "registry://docker.io/my-org",
		Only:   "att",
	}
	body, _ := json.Marshal(req)

	r := httptest.NewRequest(http.MethodPost, "/api/store/copy", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Copy(w, r)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	jobID, _ := resp["jobId"].(float64)

	// Verify --only att flag was added
	ctx := context.Background()
	job, err := handler.JobRunner.GetJob(ctx, int64(jobID))
	if err != nil {
		t.Fatalf("getting job: %v", err)
	}

	hasOnlyAtt := false
	for i, arg := range job.Args {
		if arg == "--only" && i+1 < len(job.Args) && job.Args[i+1] == "att" {
			hasOnlyAtt = true
			break
		}
	}
	if !hasOnlyAtt {
		t.Error("expected --only att flag in args")
	}

	// Clean up job
	db.Exec("DELETE FROM jobs WHERE id = ?", job.ID)
}
