package jobrunner

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"syscall"
	"time"
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	StatusQueued    JobStatus = "queued"
	StatusRunning   JobStatus = "running"
	StatusSucceeded JobStatus = "succeeded"
	StatusFailed    JobStatus = "failed"
)

// Job represents a single job execution
type Job struct {
	ID           int64
	Command      string
	Args         []string
	EnvOverrides map[string]string
	Status       JobStatus
	ExitCode     *int
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	Result       sql.NullString
}

// LogEntry represents a single log line
type LogEntry struct {
	ID        int64
	JobID     int64
	Stream    string // "stdout" or "stderr"
	Content   string
	Timestamp time.Time
}

// Runner handles job execution and log persistence
type Runner struct {
	db *sql.DB
	mu sync.Mutex
}

// New creates a new job runner
func New(db *sql.DB) *Runner {
	return &Runner{db: db}
}

// DB returns the underlying database connection
func (r *Runner) DB() *sql.DB {
	return r.db
}

// CreateJob creates a new job in the database
func (r *Runner) CreateJob(ctx context.Context, command string, args []string, envOverrides map[string]string) (*Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshaling args: %w", err)
	}

	envJSON, err := json.Marshal(envOverrides)
	if err != nil {
		return nil, fmt.Errorf("marshaling env overrides: %w", err)
	}

	var jobID int64
	err = r.db.QueryRowContext(ctx,
		`INSERT INTO jobs (command, args, env_overrides, status)
		 VALUES (?, ?, ?, ?)
		 RETURNING id`,
		command, string(argsJSON), string(envJSON), StatusQueued,
	).Scan(&jobID)
	if err != nil {
		return nil, fmt.Errorf("inserting job: %w", err)
	}

	return &Job{
		ID:      jobID,
		Command: command,
		Args:    args,
		Status:  StatusQueued,
	}, nil
}

// Start executes a job and updates its state
func (r *Runner) Start(ctx context.Context, jobID int64) error {
	// Get job details
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("getting job: %w", err)
	}

	// Update status to running
	now := time.Now()
	if err := r.updateStatus(ctx, jobID, StatusRunning, &now, nil, nil); err != nil {
		return fmt.Errorf("updating status to running: %w", err)
	}

	// Build environment - start with current env and add overrides
	baseEnv := buildEnv(job.EnvOverrides)

	// Apply settings from database as environment variables
	env, err := r.applySettingsToEnv(ctx, baseEnv, job.EnvOverrides)
	if err != nil {
		// Log but continue - settings are optional
		fmt.Printf("Warning: failed to apply settings: %v\n", err)
		env = baseEnv
	}

	// Create command
	cmd := exec.CommandContext(ctx, job.Command, job.Args...)
	cmd.Env = env
	cmd.Dir = "/data"

	// Get pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("creating stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		completedAt := time.Now()
		exitCode := -1
		_ = r.updateStatus(ctx, jobID, StatusFailed, &now, &completedAt, &exitCode)
		return fmt.Errorf("starting command: %w", err)
	}

	// Use a WaitGroup to ensure goroutines complete before returning
	done := make(chan struct{})

	// Wait for command to finish in goroutine
	go func() {
		r.monitorCompletion(ctx, jobID, cmd)
		close(done)
	}()

	// Stream stdout
	go r.streamOutput(ctx, jobID, stdout, "stdout")

	// Stream stderr
	go r.streamOutput(ctx, jobID, stderr, "stderr")

	return nil
}

// monitorCompletion waits for the command to finish and updates the job status
func (r *Runner) monitorCompletion(ctx context.Context, jobID int64, cmd *exec.Cmd) {
	err := cmd.Wait()

	completedAt := time.Now()
	var status JobStatus
	var exitCode *int

	if err != nil {
		status = StatusFailed
		if exitError, ok := err.(*exec.ExitError); ok {
			if w, ok := exitError.Sys().(syscall.WaitStatus); ok {
				code := w.ExitStatus()
				exitCode = &code
			}
		} else {
			code := -1
			exitCode = &code
		}
	} else {
		status = StatusSucceeded
		code := 0
		exitCode = &code
	}

	_ = r.updateStatus(ctx, jobID, status, nil, &completedAt, exitCode)
}

// streamOutput reads from a pipe and writes to the database
func (r *Runner) streamOutput(ctx context.Context, jobID int64, reader io.Reader, streamName string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// Redact sensitive information before storing
		redactedLine := redactSensitive(line)
		if err := r.appendLog(ctx, jobID, streamName, redactedLine); err != nil {
			// Log error but continue scanning
			fmt.Printf("Error appending log: %v\n", err)
		}
	}
	if err := scanner.Err(); err != nil {
		_ = r.appendLog(ctx, jobID, streamName, fmt.Sprintf("[stream error: %v]", err))
	}
}

// redactSensitive redacts potential sensitive information from log lines
// This includes passwords, tokens, and other credentials that might appear in output
func redactSensitive(line string) string {
	// Redact environment variable assignments with common secret names
	secretPatterns := []struct {
		pattern *regexp.Regexp
		replacement string
	}{
		// Password environment variables
		{regexp.MustCompile(`(HAULER_REGISTRY_PASSWORD=)\S+`), "$1[REDACTED]"},
		{regexp.MustCompile(`(PASSWORD=)\S+`), "$1[REDACTED]"},
		{regexp.MustCompile(`(password=)\S+`), "$1[REDACTED]"},
		// Token patterns
		{regexp.MustCompile(`(token=)\S+`), "$1[REDACTED]"},
		{regexp.MustCompile(`(TOKEN=)\S+`), "$1[REDACTED]"},
		// Basic auth patterns (in URLs)
		{regexp.MustCompile(`://[^:/]+:[^@]+@`), "://[REDACTED]:@"},
		// Bearer tokens
		{regexp.MustCompile(`(Bearer\s+)\S+`), "${1}[REDACTED]"},
		{regexp.MustCompile(`(bearer\s+)\S+`), "${1}[REDACTED]"},
		// API keys
		{regexp.MustCompile(`(api[_-]?key=)\S+`), "$1[REDACTED]"},
		{regexp.MustCompile(`(API[_-]?KEY=)\S+`), "$1[REDACTED]"},
		// Docker config auth fields
		{regexp.MustCompile(`("auth":\s*")[^"]+(")`), "${1}[REDACTED]$2"},
		{regexp.MustCompile(`("auths":\s*\{[^}]*"auth":\s*")[^"]+(")`), "${1}[REDACTED]$2"},
	}

	redacted := line
	for _, rp := range secretPatterns {
		redacted = rp.pattern.ReplaceAllString(redacted, rp.replacement)
	}

	return redacted
}

// appendLog adds a log entry to the database
func (r *Runner) appendLog(ctx context.Context, jobID int64, stream, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO job_logs (job_id, stream, content) VALUES (?, ?, ?)`,
		jobID, stream, content,
	)
	return err
}

// updateStatus updates the job status in the database
func (r *Runner) updateStatus(ctx context.Context, jobID int64, status JobStatus, startedAt, completedAt *time.Time, exitCode *int) error {
	return r.updateStatusWithResult(ctx, jobID, status, startedAt, completedAt, exitCode, "")
}

// updateStatusWithResult updates the job status and result in the database
func (r *Runner) updateStatusWithResult(ctx context.Context, jobID int64, status JobStatus, startedAt, completedAt *time.Time, exitCode *int, result string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	query := `UPDATE jobs SET status = ?`
	args := []interface{}{status}

	if startedAt != nil {
		query += `, started_at = ?`
		args = append(args, *startedAt)
	}

	if completedAt != nil {
		query += `, completed_at = ?`
		args = append(args, *completedAt)
	}

	if exitCode != nil {
		query += `, exit_code = ?`
		args = append(args, *exitCode)
	}

	if result != "" {
		query += `, result = ?`
		args = append(args, result)
	}

	query += ` WHERE id = ?`
	args = append(args, jobID)

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// UpdateResult updates just the result field for a job
func (r *Runner) UpdateResult(ctx context.Context, jobID int64, result string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.ExecContext(ctx, `UPDATE jobs SET result = ? WHERE id = ?`, result, jobID)
	return err
}

// GetJob retrieves a job by ID
func (r *Runner) GetJob(ctx context.Context, jobID int64) (*Job, error) {
	var job Job
	var argsJSON, envJSON, resultJSON sql.NullString
	var exitCode sql.NullInt64
	var startedAt, completedAt sql.NullTime

	err := r.db.QueryRowContext(ctx,
		`SELECT id, command, args, env_overrides, status, exit_code, started_at, completed_at, created_at, result
		 FROM jobs WHERE id = ?`,
		jobID,
	).Scan(
		&job.ID, &job.Command, &argsJSON, &envJSON, &job.Status,
		&exitCode, &startedAt, &completedAt, &job.CreatedAt, &resultJSON,
	)
	if err != nil {
		return nil, err
	}

	if argsJSON.Valid {
		_ = json.Unmarshal([]byte(argsJSON.String), &job.Args)
	}

	if envJSON.Valid {
		_ = json.Unmarshal([]byte(envJSON.String), &job.EnvOverrides)
	}

	job.Result = resultJSON

	if exitCode.Valid {
		code := int(exitCode.Int64)
		job.ExitCode = &code
	}

	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	return &job, nil
}

// GetLogs retrieves logs for a job, optionally after a given timestamp
func (r *Runner) GetLogs(ctx context.Context, jobID int64, since *time.Time) ([]LogEntry, error) {
	query := `SELECT id, job_id, stream, content, timestamp FROM job_logs WHERE job_id = ?`
	args := []interface{}{jobID}

	if since != nil {
		query += ` AND timestamp > ?`
		args = append(args, *since)
	}

	query += ` ORDER BY timestamp ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		if err := rows.Scan(&log.ID, &log.JobID, &log.Stream, &log.Content, &log.Timestamp); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// ListJobs retrieves all jobs, optionally filtered by status
func (r *Runner) ListJobs(ctx context.Context, status *JobStatus) ([]Job, error) {
	query := `SELECT id, command, args, env_overrides, status, exit_code, started_at, completed_at, created_at, result
	          FROM jobs`
	args := []interface{}{}

	if status != nil {
		query += ` WHERE status = ?`
		args = append(args, *status)
	}

	query += ` ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		var argsJSON, envJSON, resultJSON sql.NullString
		var exitCode sql.NullInt64
		var startedAt, completedAt sql.NullTime

		if err := rows.Scan(
			&job.ID, &job.Command, &argsJSON, &envJSON, &job.Status,
			&exitCode, &startedAt, &completedAt, &job.CreatedAt, &resultJSON,
		); err != nil {
			return nil, err
		}

		if argsJSON.Valid {
			_ = json.Unmarshal([]byte(argsJSON.String), &job.Args)
		}

		if envJSON.Valid {
			_ = json.Unmarshal([]byte(envJSON.String), &job.EnvOverrides)
		}

		job.Result = resultJSON

		if exitCode.Valid {
			code := int(exitCode.Int64)
			job.ExitCode = &code
		}

		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// buildEnv constructs the environment variables for a command
func buildEnv(envOverrides map[string]string) []string {
	// Start with current process environment
	env := append([]string{}, os.Environ()...)

	// Add/override with custom values
	for k, v := range envOverrides {
		env = append(env, k+"="+v)
	}

	return env
}

// getSettings retrieves all settings from the database
func (r *Runner) getSettings(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT key, value FROM settings`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		if value != "" {
			settings[key] = value
		}
	}

	return settings, rows.Err()
}

// applySettingsToEnv applies database settings as environment variables
func (r *Runner) applySettingsToEnv(ctx context.Context, baseEnv []string, userOverrides map[string]string) ([]string, error) {
	// Get settings from database
	settings, err := r.getSettings(ctx)
	if err != nil {
		// Log error but don't fail - settings are optional
		fmt.Printf("Warning: failed to load settings: %v\n", err)
		return baseEnv, nil
	}

	// Map of setting keys to environment variable names
	settingEnvMap := map[string]string{
		"log_level":        "HAULER_LOG_LEVEL",
		"retries":          "HAULER_RETRIES",
		"ignore_errors":    "HAULER_IGNORE_ERRORS",
		"default_platform": "HAULER_DEFAULT_PLATFORM",
		"default_key_path": "HAULER_KEY_PATH",
		"temp_dir":         "HAULER_TEMP_DIR",
	}

	// Start with base environment
	env := append([]string{}, baseEnv...)

	// Apply settings as environment variables (only if not already set in user overrides)
	for settingKey, envVar := range settingEnvMap {
		if value, ok := settings[settingKey]; ok && value != "" {
			// Check if user has already set this env var
			userSet := false
			for _, existing := range env {
				if len(existing) > len(envVar)+1 && existing[:len(envVar)+1] == envVar+"=" {
					userSet = true
					break
				}
			}
			// Also check user overrides
			if _, userOverride := userOverrides[envVar]; userOverride {
				userSet = true
			}

			// Only apply if user hasn't explicitly set it
			if !userSet {
				env = append(env, envVar+"="+value)
			}
		}
	}

	return env, nil
}
