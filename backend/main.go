package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hauler-ui/hauler-ui/backend/internal/auth"
	"github.com/hauler-ui/hauler-ui/backend/internal/config"
	"github.com/hauler-ui/hauler-ui/backend/internal/hauler"
	"github.com/hauler-ui/hauler-ui/backend/internal/jobrunner"
	"github.com/hauler-ui/hauler-ui/backend/internal/manifests"
	"github.com/hauler-ui/hauler-ui/backend/internal/registry"
	"github.com/hauler-ui/hauler-ui/backend/internal/serve"
	"github.com/hauler-ui/hauler-ui/backend/internal/settings"
	"github.com/hauler-ui/hauler-ui/backend/internal/sqlite"
	"github.com/hauler-ui/hauler-ui/backend/internal/store"
)

// startJobProcessor starts a background goroutine that processes queued jobs
func startJobProcessor(runner *jobrunner.Runner, stopCh <-chan struct{}) {
	ctx := context.Background()

	go func() {
		log.Println("Job processor goroutine started")
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				log.Println("Job processor stopped")
				return
			case <-ticker.C:
				// Look for queued jobs
				jobs, err := runner.ListJobs(ctx, nil)
				if err != nil {
					log.Printf("Error listing jobs: %v", err)
					continue
				}

				log.Printf("Job processor: found %d total jobs", len(jobs))

				// Start any queued jobs
				startedCount := 0
				for _, job := range jobs {
					if job.Status == jobrunner.StatusQueued {
						log.Printf("Starting queued job #%d: %s %v", job.ID, job.Command, job.Args)
						if err := runner.Start(ctx, job.ID); err != nil {
							log.Printf("Error starting job #%d: %v", job.ID, err)
						} else {
							startedCount++
						}
					}
				}
				if startedCount > 0 {
					log.Printf("Started %d jobs", startedCount)
				}
			}
		}
	}()
	log.Println("Job processor started")
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

	// Initialize store handler
	storeHandler := store.NewHandler(jobRunner, cfg)

	// Initialize manifests handler
	manifestsHandler := manifests.NewHandler(db.DB)

	// Initialize serve handler
	serveHandler := serve.NewHandler(cfg, db.DB)

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

	// Store endpoints
	storeHandler.RegisterRoutes(mux)

	// Manifests endpoints
	manifestsHandler.RegisterRoutes(mux)

	// Serve endpoints
	serveHandler.RegisterRoutes(mux)

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

	log.Println("Server starting on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
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
