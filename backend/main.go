package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/hauler-ui/hauler-ui/backend/internal/config"
	"github.com/hauler-ui/hauler-ui/backend/internal/jobrunner"
	"github.com/hauler-ui/hauler-ui/backend/internal/sqlite"
)

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

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/api/config", configHandler(cfg))

	// Job API endpoints
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			jobHandler.CreateJob(w, r)
		} else {
			jobHandler.ListJobs(w, r)
		}
	})
	mux.HandleFunc("/api/jobs/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a logs or stream request
		if len(r.URL.Path) > len("/api/jobs/") {
			suffix := r.URL.Path[len("/api/jobs/"):]
			if len(suffix) > 0 {
				// Look for /logs or /stream suffix
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
		http.ServeFile(w, r, "/app/web/index.html")
	})

	// Serve static files from web build directory
	fs := http.FileServer(http.Dir("/app/web"))
	mux.Handle("/assets/", fs)

	server := &http.Server{
		Addr:        ":8080",
		Handler:     mux,
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
