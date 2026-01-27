package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the sql.DB with application-specific methods
type DB struct {
	*sql.DB
}

// Open opens the SQLite database at the given path, applying migrations
func Open(path string) (*DB, error) {
	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	// Set pragmatic settings
	db.SetMaxOpenConns(1) // SQLite doesn't support multiple writers
	db.SetMaxIdleConns(1)

	if err := applyMigrations(db); err != nil {
		return nil, fmt.Errorf("applying migrations: %w", err)
	}

	return &DB{DB: db}, nil
}

// applyMigrations applies any pending migrations to the database
func applyMigrations(db *sql.DB) error {
	// Create migrations table if it doesn't exist
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Get applied migrations
	rows, err := db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("querying applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scanning migration version: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating applied migrations: %w", err)
	}

	// Get available migrations from embed FS
	subFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("getting migrations sub-filesystem: %w", err)
	}

	migrationFiles, err := fs.Glob(subFS, "*.sql")
	if err != nil {
		return fmt.Errorf("globbing migration files: %w", err)
	}

	// Sort migrations by version number
	sort.Slice(migrationFiles, func(i, j int) bool {
		vi, vj := parseVersion(migrationFiles[i]), parseVersion(migrationFiles[j])
		return vi < vj
	})

	// Apply pending migrations
	for _, file := range migrationFiles {
		version := parseVersion(file)
		if applied[version] {
			continue
		}

		content, err := fs.ReadFile(subFS, file)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", file, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for migration %d: %w", version, err)
		}

		// Execute migration
		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("executing migration %d: %w", version, err)
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", version, err)
		}
	}

	return nil
}

// parseVersion extracts the version number from a filename like "0001_name.sql"
func parseVersion(filename string) int {
	// Extract the numeric prefix
	base := strings.TrimSuffix(filename, ".sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) == 0 {
		return 0
	}
	var version int
	fmt.Sscanf(parts[0], "%d", &version)
	return version
}
