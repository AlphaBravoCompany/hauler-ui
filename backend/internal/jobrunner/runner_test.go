package jobrunner

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
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
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
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

	return db
}

func TestCreateJob(t *testing.T) {
	db := setupTestDB(t)
	runner := New(db)

	ctx := context.Background()
	job, err := runner.CreateJob(ctx, "echo", []string{"hello", "world"}, nil)
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	if job.ID == 0 {
		t.Error("expected non-zero job ID")
	}
	if job.Command != "echo" {
		t.Errorf("expected command 'echo', got %q", job.Command)
	}
	if len(job.Args) != 2 || job.Args[0] != "hello" || job.Args[1] != "world" {
		t.Errorf("unexpected args: %v", job.Args)
	}
	if job.Status != StatusQueued {
		t.Errorf("expected status %q, got %q", StatusQueued, job.Status)
	}
}

func TestGetJob(t *testing.T) {
	db := setupTestDB(t)
	runner := New(db)

	ctx := context.Background()
	created, err := runner.CreateJob(ctx, "test", []string{"arg1"}, nil)
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	fetched, err := runner.GetJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, fetched.ID)
	}
	if fetched.Command != created.Command {
		t.Errorf("expected command %q, got %q", created.Command, fetched.Command)
	}
}

func TestJobExecutionAndLogCapture(t *testing.T) {
	db := setupTestDB(t)
	runner := New(db)

	ctx := context.Background()

	// Find echo command path
	echoPath, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo command not found")
	}

	job, err := runner.CreateJob(ctx, echoPath, []string{"hello", "world"}, nil)
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	// Start the job
	if err := runner.Start(ctx, job.ID); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for job to complete
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var finalJob *Job
	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for job to complete")
		case <-ticker.C:
			finalJob, err = runner.GetJob(ctx, job.ID)
			if err != nil {
				t.Fatalf("GetJob failed: %v", err)
			}
			if finalJob.Status == StatusSucceeded || finalJob.Status == StatusFailed {
				goto done
			}
		}
	}
done:

	if finalJob.Status != StatusSucceeded {
		t.Errorf("expected status %q, got %q", StatusSucceeded, finalJob.Status)
	}
	if finalJob.ExitCode == nil || *finalJob.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %v", finalJob.ExitCode)
	}
	if finalJob.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}
	if finalJob.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}

	// Get logs
	logs, err := runner.GetLogs(ctx, job.ID, nil)
	if err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}

	if len(logs) == 0 {
		t.Fatal("expected at least one log entry")
	}

	// Check for expected output
	found := false
	for _, log := range logs {
		if log.Stream == "stdout" && strings.Contains(log.Content, "hello world") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find 'hello world' in stdout logs, got: %v", logs)
	}
}

func TestJobExecutionWithStderr(t *testing.T) {
	db := setupTestDB(t)
	runner := New(db)

	ctx := context.Background()

	// Use a command that writes to stderr
	// On Linux, we can use 'sh' with a command that writes to stderr
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh command not found")
	}

	job, err := runner.CreateJob(ctx, shPath, []string{"-c", "echo error >&2"}, nil)
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	if err := runner.Start(ctx, job.ID); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for completion
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var finalJob *Job
	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for job to complete")
		case <-ticker.C:
			finalJob, err = runner.GetJob(ctx, job.ID)
			if err != nil {
				t.Fatalf("GetJob failed: %v", err)
			}
			if finalJob.Status == StatusSucceeded || finalJob.Status == StatusFailed {
				goto done
			}
		}
	}
done:

	// Get logs
	logs, err := runner.GetLogs(ctx, job.ID, nil)
	if err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}

	// Check for stderr output
	foundStderr := false
	for _, log := range logs {
		if log.Stream == "stderr" && strings.Contains(log.Content, "error") {
			foundStderr = true
			break
		}
	}
	if !foundStderr {
		t.Errorf("expected to find stderr output, got: %v", logs)
	}
}

func TestJobExecutionFailure(t *testing.T) {
	db := setupTestDB(t)
	runner := New(db)

	ctx := context.Background()

	// Use a command that will fail
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh command not found")
	}

	job, err := runner.CreateJob(ctx, shPath, []string{"-c", "exit 42"}, nil)
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	if err := runner.Start(ctx, job.ID); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for completion
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var finalJob *Job
	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for job to complete")
		case <-ticker.C:
			finalJob, err = runner.GetJob(ctx, job.ID)
			if err != nil {
				t.Fatalf("GetJob failed: %v", err)
			}
			if finalJob.Status == StatusSucceeded || finalJob.Status == StatusFailed {
				goto done
			}
		}
	}
done:

	if finalJob.Status != StatusFailed {
		t.Errorf("expected status %q, got %q", StatusFailed, finalJob.Status)
	}
	if finalJob.ExitCode == nil || *finalJob.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %v", finalJob.ExitCode)
	}
}

func TestListJobs(t *testing.T) {
	db := setupTestDB(t)
	runner := New(db)

	ctx := context.Background()

	// Create some jobs
	_, _ = runner.CreateJob(ctx, "cmd1", nil, nil)
	_, _ = runner.CreateJob(ctx, "cmd2", nil, nil)
	job3, _ := runner.CreateJob(ctx, "cmd3", nil, nil)

	// List all jobs
	jobs, err := runner.ListJobs(ctx, nil)
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}

	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(jobs))
	}

	// List by status
	s := StatusQueued
	queuedJobs, err := runner.ListJobs(ctx, &s)
	if err != nil {
		t.Fatalf("ListJobs with status failed: %v", err)
	}

	if len(queuedJobs) != 3 {
		t.Errorf("expected 3 queued jobs, got %d", len(queuedJobs))
	}

	// Verify IDs are unique
	ids := make(map[int64]bool)
	for _, j := range jobs {
		if ids[j.ID] {
			t.Errorf("duplicate job ID: %d", j.ID)
		}
		ids[j.ID] = true
	}

	// Verify job3 is in the list
	found := false
	for _, j := range jobs {
		if j.ID == job3.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("job3 not found in jobs list")
	}
}

func TestGetLogsWithSince(t *testing.T) {
	db := setupTestDB(t)
	runner := New(db)

	ctx := context.Background()

	echoPath, err := exec.LookPath("echo")
	if err != nil {
		t.Skip("echo command not found")
	}

	job, err := runner.CreateJob(ctx, echoPath, []string{"test"}, nil)
	if err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	if err := runner.Start(ctx, job.ID); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for completion
	for i := 0; i < 50; i++ {
		finalJob, _ := runner.GetJob(ctx, job.ID)
		if finalJob.Status == StatusSucceeded || finalJob.Status == StatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Get all logs
	allLogs, err := runner.GetLogs(ctx, job.ID, nil)
	if err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}

	if len(allLogs) == 0 {
		t.Fatal("expected at least one log entry")
	}

	// Get logs after the first timestamp
	since := allLogs[0].Timestamp
	partialLogs, err := runner.GetLogs(ctx, job.ID, &since)
	if err != nil {
		t.Fatalf("GetLogs with since failed: %v", err)
	}

	// Should have fewer logs (all except possibly ones at exactly the since timestamp)
	if len(partialLogs) > len(allLogs) {
		t.Errorf("expected partial logs (%d) <= all logs (%d)", len(partialLogs), len(allLogs))
	}
}
