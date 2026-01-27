package sqlite

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationsApplyOnEmptyDB(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database (should apply migrations)
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify schema_migrations table exists and has our migration
	var migrationCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	if migrationCount != 1 {
		t.Errorf("Expected 1 migration, got %d", migrationCount)
	}

	// Verify all tables exist
	tables := []string{"settings", "jobs", "job_logs", "saved_manifests", "serve_processes"}
	for _, table := range tables {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count); err != nil {
			t.Fatalf("Failed to check table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("Table %s does not exist", table)
		}
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database first time
	db1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database first time: %v", err)
	}
	db1.Close()

	// Open database second time (should re-apply migrations without error)
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database second time: %v", err)
	}
	defer db2.Close()

	// Verify migrations weren't duplicated
	var migrationCount int
	if err := db2.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatalf("Failed to query schema_migrations: %v", err)
	}
	if migrationCount != 1 {
		t.Errorf("Expected 1 migration after reopen, got %d", migrationCount)
	}
}

func TestDefaultSettingsInserted(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify default settings exist
	defaultKeys := []string{"log_level", "retries", "ignore_errors", "default_platform", "default_key_path"}
	for _, key := range defaultKeys {
		var value string
		if err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value); err != nil {
			if err == sql.ErrNoRows {
				t.Errorf("Default setting %s was not inserted", key)
			} else {
				t.Fatalf("Failed to query setting %s: %v", key, err)
			}
		}
	}
}

func TestJobsTableSchema(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test inserting and querying a job
	result, err := db.Exec("INSERT INTO jobs (command, args, status) VALUES (?, ?, ?)",
		"hauler", `["store", "add"]`, "queued")
	if err != nil {
		t.Fatalf("Failed to insert job: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get last insert id: %v", err)
	}

	var command string
	var status string
	if err := db.QueryRow("SELECT command, status FROM jobs WHERE id = ?", id).Scan(&command, &status); err != nil {
		t.Fatalf("Failed to query job: %v", err)
	}

	if command != "hauler" {
		t.Errorf("Expected command 'hauler', got '%s'", command)
	}
	if status != "queued" {
		t.Errorf("Expected status 'queued', got '%s'", status)
	}
}

func TestJobLogsTableSchema(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a job first
	jobResult, err := db.Exec("INSERT INTO jobs (command, status) VALUES (?, ?)", "hauler", "running")
	if err != nil {
		t.Fatalf("Failed to insert job: %v", err)
	}
	jobID, _ := jobResult.LastInsertId()

	// Test inserting and querying job logs
	_, err = db.Exec("INSERT INTO job_logs (job_id, stream, content) VALUES (?, ?, ?)",
		jobID, "stdout", "test log line")
	if err != nil {
		t.Fatalf("Failed to insert job log: %v", err)
	}

	var stream, content string
	if err := db.QueryRow("SELECT stream, content FROM job_logs WHERE job_id = ?", jobID).Scan(&stream, &content); err != nil {
		t.Fatalf("Failed to query job log: %v", err)
	}

	if stream != "stdout" {
		t.Errorf("Expected stream 'stdout', got '%s'", stream)
	}
	if content != "test log line" {
		t.Errorf("Expected content 'test log line', got '%s'", content)
	}
}

func TestSavedManifestsTableSchema(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test inserting and querying a saved manifest
	yamlContent := "apiVersion: content.hauler.cattle.io/v1\nkind: ImageConfig"
	result, err := db.Exec("INSERT INTO saved_manifests (name, description, yaml_content) VALUES (?, ?, ?)",
		"test-manifest", "A test manifest", yamlContent)
	if err != nil {
		t.Fatalf("Failed to insert manifest: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get last insert id: %v", err)
	}

	var name, content string
	if err := db.QueryRow("SELECT name, yaml_content FROM saved_manifests WHERE id = ?", id).Scan(&name, &content); err != nil {
		t.Fatalf("Failed to query manifest: %v", err)
	}

	if name != "test-manifest" {
		t.Errorf("Expected name 'test-manifest', got '%s'", name)
	}
	if content != yamlContent {
		t.Errorf("Expected yaml content '%s', got '%s'", yamlContent, content)
	}
}

func TestServeProcessesTableSchema(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test inserting and querying a serve process
	args := `{"port": 5000, "readonly": true}`
	result, err := db.Exec("INSERT INTO serve_processes (serve_type, pid, port, args, status) VALUES (?, ?, ?, ?, ?)",
		"registry", 12345, 5000, args, "running")
	if err != nil {
		t.Fatalf("Failed to insert serve process: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get last insert id: %v", err)
	}

	var serveType, status string
	var port int
	if err := db.QueryRow("SELECT serve_type, port, status FROM serve_processes WHERE id = ?", id).Scan(&serveType, &port, &status); err != nil {
		t.Fatalf("Failed to query serve process: %v", err)
	}

	if serveType != "registry" {
		t.Errorf("Expected serve_type 'registry', got '%s'", serveType)
	}
	if port != 5000 {
		t.Errorf("Expected port 5000, got %d", port)
	}
	if status != "running" {
		t.Errorf("Expected status 'running', got '%s'", status)
	}
}

func TestJobLogsIndexExists(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify the index exists
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_job_logs_job_id'").Scan(&count); err != nil {
		t.Fatalf("Failed to check for index: %v", err)
	}
	if count != 1 {
		t.Errorf("Index idx_job_logs_job_id does not exist")
	}
}

func TestDatabasePathCreation(t *testing.T) {
	// Create a temporary directory for the database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "app.db")

	// Open database (should create parent directory)
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify the database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file was not created at %s", dbPath)
	}
}
