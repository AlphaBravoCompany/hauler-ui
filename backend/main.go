package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hauler-ui/hauler-ui/backend/internal/auth"
	"github.com/hauler-ui/hauler-ui/backend/internal/config"
	"github.com/hauler-ui/hauler-ui/backend/internal/hauler"
	"github.com/hauler-ui/hauler-ui/backend/internal/hauls"
	"github.com/hauler-ui/hauler-ui/backend/internal/jobrunner"
	"github.com/hauler-ui/hauler-ui/backend/internal/manifests"
	"github.com/hauler-ui/hauler-ui/backend/internal/publish"
	"github.com/hauler-ui/hauler-ui/backend/internal/registry"
	"github.com/hauler-ui/hauler-ui/backend/internal/serve"
	"github.com/hauler-ui/hauler-ui/backend/internal/settings"
	"github.com/hauler-ui/hauler-ui/backend/internal/sqlite"
	"github.com/hauler-ui/hauler-ui/backend/internal/store"
)

// maxConcurrentJobs caps how many jobs run at once so a burst of queued
// operations does not exhaust memory/disk on a single-instance deployment.
func maxConcurrentJobs() int {
	if v := os.Getenv("HAULER_UI_MAX_CONCURRENT_JOBS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 2
}

// startJobProcessor starts a background goroutine that processes queued jobs,
// honoring a concurrency limit.
func startJobProcessor(runner *jobrunner.Runner, stopCh <-chan struct{}) {
	ctx := context.Background()
	limit := maxConcurrentJobs()

	go func() {
		log.Printf("Job processor goroutine started (max concurrent: %d)", limit)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				log.Println("Job processor stopped")
				return
			case <-ticker.C:
				jobs, err := runner.ListJobs(ctx, nil)
				if err != nil {
					log.Printf("Error listing jobs: %v", err)
					continue
				}

				// Count what is already running and only fill up to the limit.
				running := 0
				for _, job := range jobs {
					if job.Status == jobrunner.StatusRunning {
						running++
					}
				}

				for _, job := range jobs {
					if running >= limit {
						break
					}
					if job.Status == jobrunner.StatusQueued {
						log.Printf("Starting queued job #%d: %s %v", job.ID, job.Command, job.Args)
						if err := runner.Start(ctx, job.ID); err != nil {
							log.Printf("Error starting job #%d: %v", job.ID, err)
						} else {
							running++
						}
					}
				}
			}
		}
	}()
	log.Println("Job processor started")
}

// cleanupOnBoot resets state left over from a previous run: jobs stuck in
// "running" (the process died mid-job) are marked failed, and stale serve
// process rows (dead PIDs) are marked stopped so the UI reflects reality.
func cleanupOnBoot(db *sql.DB) {
	if res, err := db.Exec(`UPDATE jobs SET status = 'failed', completed_at = CURRENT_TIMESTAMP WHERE status = 'running'`); err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			log.Printf("Boot cleanup: marked %d interrupted job(s) as failed", n)
		}
	}
	if res, err := db.Exec(`UPDATE serve_processes SET status = 'stopped', stopped_at = CURRENT_TIMESTAMP WHERE status = 'running'`); err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			log.Printf("Boot cleanup: marked %d stale serve process(es) as stopped", n)
		}
	}
}

func main() {
	cfg := config.Load()

	// Initialize SQLite database
	db, err := sqlite.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Printf("Database initialized: %s", cfg.DatabasePath)

	// Reset state left over from a previous run before anything starts.
	cleanupOnBoot(db.DB)

	// Initialize job runner
	jobRunner := jobrunner.New(db.DB)
	jobHandler := jobrunner.NewHandler(jobRunner, cfg)

	// Start background job processor
	stopCh := make(chan struct{})
	defer close(stopCh)
	startJobProcessor(jobRunner, stopCh)

	// Initialize hauler detector
	haulerBinary := getEnv("HAULER_BINARY", "hauler")
	haulerDetector := hauler.New(haulerBinary)
	haulerHandler := hauler.NewHandler(haulerDetector)

	// Initialize registry handler
	registryHandler := registry.NewHandler(jobRunner, cfg)

	// Initialize haul service and ensure a default haul exists on first boot
	haulService := hauls.NewService(db.DB, cfg)
	if _, err := haulService.EnsureDefault(context.Background()); err != nil {
		log.Printf("Warning: failed to ensure default haul: %v", err)
	}
	haulsHandler := hauls.NewHandler(haulService)

	// Initialize store handler
	storeHandler := store.NewHandler(jobRunner, cfg, haulService)

	// Initialize manifests handler
	manifestsHandler := manifests.NewHandler(db.DB, haulService)

	// Initialize serve handler
	serveHandler := serve.NewHandler(cfg, db.DB, haulService)

	// Initialize publish manager (host-routed registries + path-routed files)
	publishManager := publish.NewManager(cfg, db.DB, haulService)
	publishHandler := publish.NewHandler(publishManager, haulService)
	publishManager.RestoreOnBoot(context.Background())

	// Initialize settings handler
	settingsHandler := settings.NewHandler(db.DB)

	// Initialize auth manager and handler
	authManager := auth.NewManager(db.DB, cfg)
	authHandler := auth.NewHandler(authManager)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/api/config", configHandler(cfg))

	// Auth endpoints (public)
	authHandler.RegisterRoutes(mux)

	// Hauler capabilities endpoints
	haulerHandler.RegisterRoutes(mux)

	// Registry endpoints
	registryHandler.RegisterRoutes(mux)

	// Haul endpoints
	haulsHandler.RegisterRoutes(mux)

	// Store endpoints
	storeHandler.RegisterRoutes(mux)

	// Manifests endpoints
	manifestsHandler.RegisterRoutes(mux)

	// Serve endpoints
	serveHandler.RegisterRoutes(mux)

	// Publish endpoints (routes table, publish/unpublish) and /h/ file serving
	publishHandler.RegisterRoutes(mux)

	// Settings endpoints
	settingsHandler.RegisterRoutes(mux)

	// Job API endpoints
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			jobHandler.CreateJob(w, r)
		case http.MethodDelete:
			jobHandler.DeleteAllJobs(w, r)
		default:
			jobHandler.ListJobs(w, r)
		}
	})
	mux.HandleFunc("/api/jobs/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a logs, stream, or cleanup request
		if len(r.URL.Path) > len("/api/jobs/") {
			suffix := r.URL.Path[len("/api/jobs/"):]
			if len(suffix) > 0 {
				// Look for /logs, /stream, or /cleanup suffix
				for i, c := range suffix {
					if c == '/' {
						sub := suffix[i:]
						if sub == "/logs" {
							jobHandler.GetJobLogs(w, r)
							return
						}
						if sub == "/stream" {
							jobHandler.StreamJobLogs(w, r)
							return
						}
						if sub == "/cleanup" && r.Method == http.MethodPost {
							jobHandler.CleanupStaleJob(w, r)
							return
						}
					}
				}
				// No special suffix, treat as get job
				jobHandler.GetJob(w, r)
				return
			}
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "./web/index.html")
	})

	// Serve static files from web build directory
	fs := http.FileServer(http.Dir("./web"))
	mux.Handle("/assets/", fs)
	mux.HandleFunc("/hauler-logo.svg", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/hauler-logo.svg")
	})
	mux.HandleFunc("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/favicon.svg")
	})

	// Wrap mux with auth middleware
	handlerWithAuth := authManager.Middleware(mux)

	server := &http.Server{
		Addr:        ":8080",
		Handler:     handlerWithAuth,
		ReadTimeout: 5 * time.Second,
	}

	// Start the single host-routed registry proxy listener for published hauls.
	// Runs unauthenticated (registry clients use their own auth), on its own port.
	registryAddr := ":" + getEnv("HAULER_UI_REGISTRY_PORT", "5000")
	go publishManager.StartRegistryListener(registryAddr)

	// Run the UI/API server until a termination signal arrives.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Println("Server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutdown signal received; stopping...")

	// Stop spawned hauler children so they are not orphaned, then drain HTTP.
	publishManager.StopAll()
	serveHandler.StopAll()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
	log.Println("Shutdown complete")
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func configHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(cfg.ToMap())
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
