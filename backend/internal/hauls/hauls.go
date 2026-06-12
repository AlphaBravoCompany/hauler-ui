// Package hauls manages first-class haul workspaces. Each haul owns an isolated
// hauler store directory on disk, allowing multiple hauls to coexist and be
// operated on independently.
package hauls

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
)

// Haul is a named, isolated workspace backed by its own store directory.
type Haul struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	StoreDir    string    `json:"storeDir"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ArchivesDir returns the directory where this haul's built .tar.zst archives live.
func (h *Haul) ArchivesDir() string {
	// store_dir is <base>/<slug>/store; archives sit alongside it.
	return filepath.Join(filepath.Dir(h.StoreDir), "archives")
}

// Service provides CRUD and filesystem management for hauls.
type Service struct {
	db  *sql.DB
	cfg *config.Config
}

// NewService creates a haul service.
func NewService(db *sql.DB, cfg *config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

// baseDir is the root under which all per-haul directories are created.
func (s *Service) baseDir() string {
	return filepath.Join(s.cfg.DataDir, "hauls")
}

var slugInvalid = regexp.MustCompile(`[^a-z0-9-]+`)

// slugify converts a display name into a filesystem-safe slug.
func slugify(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = slugInvalid.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "haul"
	}
	return slug
}

// EnsureDefault guarantees at least one haul exists, creating a "Default" haul
// on first boot so the app is never empty.
func (s *Service) EnsureDefault(ctx context.Context) (*Haul, error) {
	hauls, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(hauls) > 0 {
		return &hauls[0], nil
	}
	return s.Create(ctx, "Default", "Default haul workspace")
}

// scanHaul reads a single Haul row from the given scanner.
func scanHaul(row interface{ Scan(...any) error }) (*Haul, error) {
	var h Haul
	var desc sql.NullString
	if err := row.Scan(&h.ID, &h.Name, &h.Slug, &desc, &h.StoreDir, &h.CreatedAt, &h.UpdatedAt); err != nil {
		return nil, err
	}
	h.Description = desc.String
	return &h, nil
}

// List returns all hauls, newest first.
func (s *Service) List(ctx context.Context) ([]Haul, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, slug, description, store_dir, created_at, updated_at
		 FROM hauls ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hauls []Haul
	for rows.Next() {
		h, err := scanHaul(rows)
		if err != nil {
			return nil, err
		}
		hauls = append(hauls, *h)
	}
	return hauls, rows.Err()
}

// Get returns a single haul by id.
func (s *Service) Get(ctx context.Context, id int64) (*Haul, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, slug, description, store_dir, created_at, updated_at
		 FROM hauls WHERE id = ?`, id)
	return scanHaul(row)
}

// GetBySlug returns a single haul by its filesystem-safe slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Haul, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, slug, description, store_dir, created_at, updated_at
		 FROM hauls WHERE slug = ?`, slug)
	return scanHaul(row)
}

// Create makes a new haul, initializing its store directory as an empty OCI layout.
func (s *Service) Create(ctx context.Context, name, description string) (*Haul, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Derive a unique slug.
	base := slugify(name)
	slug := base
	for i := 2; ; i++ {
		var exists int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM hauls WHERE slug = ?`, slug).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}

	storeDir := filepath.Join(s.baseDir(), slug, "store")
	archivesDir := filepath.Join(s.baseDir(), slug, "archives")
	if err := initStoreDir(storeDir); err != nil {
		return nil, fmt.Errorf("initializing store directory: %w", err)
	}
	if err := os.MkdirAll(archivesDir, 0755); err != nil {
		return nil, fmt.Errorf("creating archives directory: %w", err)
	}

	var id int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO hauls (name, slug, description, store_dir)
		 VALUES (?, ?, ?, ?) RETURNING id`,
		name, slug, description, storeDir,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("inserting haul: %w", err)
	}

	return s.Get(ctx, id)
}

// Update changes a haul's name and/or description. The slug and store directory
// are stable for the lifetime of the haul.
func (s *Service) Update(ctx context.Context, id int64, name, description *string) (*Haul, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	newName := existing.Name
	if name != nil && strings.TrimSpace(*name) != "" {
		newName = strings.TrimSpace(*name)
	}
	newDesc := existing.Description
	if description != nil {
		newDesc = *description
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE hauls SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		newName, newDesc, id)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Delete removes a haul, its store directory, archives, and tracked contents.
func (s *Service) Delete(ctx context.Context, id int64) error {
	h, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	// Remove the haul's directory tree (store + archives).
	haulDir := filepath.Dir(h.StoreDir)
	if err := os.RemoveAll(haulDir); err != nil {
		return fmt.Errorf("removing haul directory: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM store_contents WHERE haul_id = ?`, id); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM hauls WHERE id = ?`, id); err != nil {
		return err
	}
	return nil
}

// initStoreDir creates an empty OCI layout so hauler can operate on a fresh haul.
func initStoreDir(storeDir string) error {
	blobsDir := filepath.Join(storeDir, "blobs", "sha256")
	if err := os.MkdirAll(blobsDir, 0755); err != nil {
		return err
	}
	ociLayoutPath := filepath.Join(storeDir, "oci-layout")
	if _, err := os.Stat(ociLayoutPath); os.IsNotExist(err) {
		if err := os.WriteFile(ociLayoutPath, []byte(`{"imageLayoutVersion": "1.0.0"}`), 0644); err != nil {
			return err
		}
	}
	return nil
}
