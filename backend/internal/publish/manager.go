// Package publish exposes multiple hauls through hauler-ui's single front door:
// a host-routed reverse proxy for per-haul registries and path-routed file
// serving straight from each haul's store.
package publish

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
	"github.com/hauler-ui/hauler-ui/backend/internal/hauls"
)

// published is a live, exposed haul: an internal readonly registry process plus
// the virtual hostname that routes to it.
type published struct {
	HaulID    int64
	Slug      string
	Hostname  string
	Port      int // internal registry port (127.0.0.1:Port)
	StartedAt time.Time
	cmd       *exec.Cmd
}

// Manager owns the set of published hauls and their internal registry processes.
type Manager struct {
	cfg   *config.Config
	db    *sql.DB
	hauls *hauls.Service

	mu     sync.RWMutex
	byHaul map[int64]*published
	proxy  *httputil.ReverseProxy
	tls    *tlsState
}

// NewManager creates a publish manager.
func NewManager(cfg *config.Config, db *sql.DB, haulSvc *hauls.Service) *Manager {
	m := &Manager{
		cfg:    cfg,
		db:     db,
		hauls:  haulSvc,
		byHaul: make(map[int64]*published),
	}
	// Single reverse proxy whose Director resolves the target per request from
	// the incoming Host header.
	m.proxy = &httputil.ReverseProxy{
		Director:      m.director,
		FlushInterval: 250 * time.Millisecond, // stream blobs, don't buffer
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("registry proxy error for host %q: %v", r.Host, err)
			http.Error(w, "registry unavailable", http.StatusBadGateway)
		},
	}
	m.bootstrapTLS()
	return m
}

// hostHaulKey is used to pass the resolved target through the request context.
type ctxKey string

const targetKey ctxKey = "publishTarget"

// director rewrites a proxied request to the internal registry resolved from Host.
func (m *Manager) director(r *http.Request) {
	if target, ok := r.Context().Value(targetKey).(string); ok {
		r.URL.Scheme = "http"
		r.URL.Host = target
	}
}

// RegistryDomain is the optional base domain for subdomain routing.
func (m *Manager) registryDomain() string {
	return os.Getenv("HAULER_UI_REGISTRY_DOMAIN")
}

// hostnameFor derives the virtual host for a haul: an explicit override, else
// "<slug>.<domain>" when a base domain is configured, else the bare slug.
func (m *Manager) hostnameFor(slug, override string) string {
	if override != "" {
		return strings.ToLower(override)
	}
	if d := m.registryDomain(); d != "" {
		return strings.ToLower(slug + "." + d)
	}
	return strings.ToLower(slug)
}

// freePort asks the OS for an unused TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Publish starts (or returns the existing) internal registry for a haul and
// registers its virtual host. hostnameOverride may be empty.
func (m *Manager) Publish(ctx context.Context, haulID int64, hostnameOverride string) (*published, error) {
	haul, err := m.hauls.Get(ctx, haulID)
	if err != nil {
		return nil, fmt.Errorf("resolving haul: %w", err)
	}

	m.mu.Lock()
	if existing, ok := m.byHaul[haulID]; ok {
		m.mu.Unlock()
		return existing, nil
	}
	m.mu.Unlock()

	port, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("allocating port: %w", err)
	}

	registryDir := filepath.Join(filepath.Dir(haul.StoreDir), "registry")
	args := []string{
		"store", "serve", "registry",
		"--readonly",
		"--port", fmt.Sprintf("%d", port),
		"--store", haul.StoreDir,
		"--directory", registryDir,
	}
	cmd := exec.Command("hauler", args...)
	cmd.Dir = m.cfg.DataDir
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting internal registry: %w", err)
	}

	p := &published{
		HaulID:    haulID,
		Slug:      haul.Slug,
		Hostname:  m.hostnameFor(haul.Slug, hostnameOverride),
		Port:      port,
		StartedAt: time.Now(),
		cmd:       cmd,
	}

	m.mu.Lock()
	m.byHaul[haulID] = p
	m.mu.Unlock()

	// Persist so we can restore on boot and surface in the routes table.
	_, _ = m.db.ExecContext(ctx, `DELETE FROM serve_processes WHERE haul_id = ? AND role = 'published'`, haulID)
	_, err = m.db.ExecContext(ctx, `
		INSERT INTO serve_processes (serve_type, pid, port, args, status, haul_id, role, hostname)
		VALUES ('registry', ?, ?, '{}', 'running', ?, 'published', ?)
	`, cmd.Process.Pid, port, haulID, p.Hostname)
	if err != nil {
		log.Printf("publish: failed to persist record for haul %d: %v", haulID, err)
	}

	go m.monitor(p)
	log.Printf("published haul %d (%s) at host %q -> 127.0.0.1:%d", haulID, haul.Slug, p.Hostname, port)
	return p, nil
}

// monitor reaps the internal registry process and clears its registration.
func (m *Manager) monitor(p *published) {
	_ = p.cmd.Wait()
	m.mu.Lock()
	if cur, ok := m.byHaul[p.HaulID]; ok && cur == p {
		delete(m.byHaul, p.HaulID)
	}
	m.mu.Unlock()
	_, _ = m.db.Exec(`UPDATE serve_processes SET status = 'stopped', stopped_at = CURRENT_TIMESTAMP WHERE haul_id = ? AND role = 'published'`, p.HaulID)
}

// Unpublish stops a haul's internal registry and removes its route.
func (m *Manager) Unpublish(ctx context.Context, haulID int64) error {
	m.mu.Lock()
	p, ok := m.byHaul[haulID]
	if ok {
		delete(m.byHaul, haulID)
	}
	m.mu.Unlock()

	if ok && p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
	}
	_, err := m.db.ExecContext(ctx, `DELETE FROM serve_processes WHERE haul_id = ? AND role = 'published'`, haulID)
	return err
}

// Route describes one published haul for the routes table.
type Route struct {
	HaulID    int64  `json:"haulId"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Hostname  string `json:"hostname"`
	Port      int    `json:"port"`
	Status    string `json:"status"`
	StartedAt string `json:"startedAt"`
}

// List returns the current publish routes.
func (m *Manager) List(ctx context.Context) []Route {
	m.mu.RLock()
	defer m.mu.RUnlock()
	routes := make([]Route, 0, len(m.byHaul))
	for _, p := range m.byHaul {
		name := p.Slug
		if h, err := m.hauls.Get(ctx, p.HaulID); err == nil {
			name = h.Name
		}
		routes = append(routes, Route{
			HaulID:    p.HaulID,
			Slug:      p.Slug,
			Name:      name,
			Hostname:  p.Hostname,
			Port:      p.Port,
			Status:    "running",
			StartedAt: p.StartedAt.Format(time.RFC3339),
		})
	}
	return routes
}

// IsPublished reports whether a haul is currently published.
func (m *Manager) IsPublished(haulID int64) (*published, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.byHaul[haulID]
	return p, ok
}

// resolveHost maps an incoming Host header to a published haul's internal target.
func (m *Manager) resolveHost(host string) (*published, bool) {
	host = strings.ToLower(host)
	if i := strings.IndexByte(host, ':'); i != -1 {
		host = host[:i] // strip port
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.byHaul {
		if p.Hostname == host {
			return p, true
		}
	}
	return nil, false
}

// RegistryProxyHandler host-routes registry traffic to the matching internal
// registry. Mount this on the dedicated registry listener.
func (m *Manager) RegistryProxyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := m.resolveHost(r.Host)
		if !ok {
			http.Error(w, fmt.Sprintf("no published haul for host %q", r.Host), http.StatusNotFound)
			return
		}
		target := fmt.Sprintf("127.0.0.1:%d", p.Port)
		ctx := context.WithValue(r.Context(), targetKey, target)
		m.proxy.ServeHTTP(w, r.WithContext(ctx))
	})
}

// StartRegistryListener binds the single host-routed registry port and serves
// the proxy. Serves HTTPS using the configured (or self-signed) certificate so
// clusters can pull over TLS; only falls back to plain HTTP if no certificate
// could be prepared at all. Runs until the process exits.
func (m *Manager) StartRegistryListener(addr string) {
	srv := &http.Server{Addr: addr, Handler: m.RegistryProxyHandler()}
	if m.hasTLS() {
		srv.TLSConfig = &tls.Config{GetCertificate: m.getCertificate}
		log.Printf("registry proxy listening on %s (host-routed, TLS: %s)", addr, m.TLSStatus().Source)
		if err := srv.ListenAndServeTLS("", ""); err != nil {
			log.Printf("registry proxy listener stopped: %v", err)
		}
		return
	}
	log.Printf("registry proxy listening on %s (host-routed, plaintext)", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("registry proxy listener stopped: %v", err)
	}
}

// RestoreOnBoot re-publishes hauls that were published before a restart.
func (m *Manager) RestoreOnBoot(ctx context.Context) {
	rows, err := m.db.QueryContext(ctx, `SELECT DISTINCT haul_id, hostname FROM serve_processes WHERE role = 'published'`)
	if err != nil {
		log.Printf("publish restore: query failed: %v", err)
		return
	}
	type want struct {
		haulID   int64
		hostname sql.NullString
	}
	var wants []want
	for rows.Next() {
		var wnt want
		if err := rows.Scan(&wnt.haulID, &wnt.hostname); err == nil {
			wants = append(wants, wnt)
		}
	}
	rows.Close()

	for _, wnt := range wants {
		// Clear the stale row, then start a fresh process.
		_, _ = m.db.ExecContext(ctx, `DELETE FROM serve_processes WHERE haul_id = ? AND role = 'published'`, wnt.haulID)
		if _, err := m.Publish(ctx, wnt.haulID, wnt.hostname.String); err != nil {
			log.Printf("publish restore: haul %d failed: %v", wnt.haulID, err)
		}
	}
}
