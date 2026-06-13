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

	mu           sync.RWMutex
	byHaul       map[int64]*published
	desired      map[int64]string // haulID -> hostname that should stay published
	shuttingDown bool
	proxy        *httputil.ReverseProxy
	tls          *tlsState
}

// NewManager creates a publish manager.
func NewManager(cfg *config.Config, db *sql.DB, haulSvc *hauls.Service) *Manager {
	m := &Manager{
		cfg:     cfg,
		db:      db,
		hauls:   haulSvc,
		byHaul:  make(map[int64]*published),
		desired: make(map[int64]string),
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
// records it as desired so it is kept alive (auto-restarted on crash).
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
	hostname := m.hostnameFor(haul.Slug, hostnameOverride)
	m.desired[haulID] = hostname
	m.mu.Unlock()

	return m.startRegistry(haul, hostname)
}

// startRegistry launches one internal registry process for a haul and begins
// monitoring it. Callers must have recorded the haul in m.desired first.
func (m *Manager) startRegistry(haul *hauls.Haul, hostname string) (*published, error) {
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
		HaulID:    haul.ID,
		Slug:      haul.Slug,
		Hostname:  hostname,
		Port:      port,
		StartedAt: time.Now(),
		cmd:       cmd,
	}

	m.mu.Lock()
	m.byHaul[haul.ID] = p
	m.mu.Unlock()

	// Persist so we can restore on boot and surface in the routes table.
	_, _ = m.db.Exec(`DELETE FROM serve_processes WHERE haul_id = ? AND role = 'published'`, haul.ID)
	if _, err := m.db.Exec(`
		INSERT INTO serve_processes (serve_type, pid, port, args, status, haul_id, role, hostname)
		VALUES ('registry', ?, ?, '{}', 'running', ?, 'published', ?)
	`, cmd.Process.Pid, port, haul.ID, hostname); err != nil {
		log.Printf("publish: failed to persist record for haul %d: %v", haul.ID, err)
	}

	go m.monitor(p)
	log.Printf("published haul %d (%s) at host %q -> 127.0.0.1:%d", haul.ID, haul.Slug, hostname, port)
	return p, nil
}

// monitor reaps the internal registry process and, unless the haul was
// intentionally unpublished or the manager is shutting down, restarts it so a
// published haul stays available after a crash.
func (m *Manager) monitor(p *published) {
	_ = p.cmd.Wait()

	m.mu.Lock()
	if cur, ok := m.byHaul[p.HaulID]; ok && cur == p {
		delete(m.byHaul, p.HaulID)
	}
	_, stillDesired := m.desired[p.HaulID]
	shuttingDown := m.shuttingDown
	m.mu.Unlock()

	if shuttingDown || !stillDesired {
		_, _ = m.db.Exec(`UPDATE serve_processes SET status = 'stopped', stopped_at = CURRENT_TIMESTAMP WHERE haul_id = ? AND role = 'published'`, p.HaulID)
		return
	}

	// Unexpected exit: attempt to restart with backoff.
	log.Printf("publish: registry for haul %d (%s) exited unexpectedly; restarting", p.HaulID, p.Slug)
	for attempt := 1; attempt <= 3; attempt++ {
		time.Sleep(time.Duration(attempt) * time.Second)

		m.mu.RLock()
		hostname, want := m.desired[p.HaulID]
		down := m.shuttingDown
		m.mu.RUnlock()
		if down || !want {
			return // unpublished or shutting down during backoff
		}

		haul, err := m.hauls.Get(context.Background(), p.HaulID)
		if err != nil {
			continue
		}
		if _, err := m.startRegistry(haul, hostname); err == nil {
			return
		}
		log.Printf("publish: restart attempt %d for haul %d failed", attempt, p.HaulID)
	}
	log.Printf("publish: giving up restarting haul %d after 3 attempts", p.HaulID)
	_, _ = m.db.Exec(`UPDATE serve_processes SET status = 'stopped', stopped_at = CURRENT_TIMESTAMP WHERE haul_id = ? AND role = 'published'`, p.HaulID)
}

// Unpublish stops a haul's internal registry and removes its route.
func (m *Manager) Unpublish(ctx context.Context, haulID int64) error {
	m.mu.Lock()
	delete(m.desired, haulID)
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

// StopAll terminates every internal registry process (used on graceful
// shutdown). It marks the manager as shutting down so monitors do not restart.
func (m *Manager) StopAll() {
	m.mu.Lock()
	m.shuttingDown = true
	procs := make([]*published, 0, len(m.byHaul))
	for _, p := range m.byHaul {
		procs = append(procs, p)
	}
	m.mu.Unlock()

	for _, p := range procs {
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Signal(syscall.SIGTERM)
		}
	}
	if len(procs) > 0 {
		log.Printf("publish: sent SIGTERM to %d internal registry process(es)", len(procs))
	}
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
