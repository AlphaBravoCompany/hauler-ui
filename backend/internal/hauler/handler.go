package hauler

import (
	"encoding/json"
	"log"
	"net/http"
)

// Handler handles HTTP requests for hauler capabilities
type Handler struct {
	detector *Detector
}

// NewHandler creates a new hauler handler
func NewHandler(detector *Detector) *Handler {
	return &Handler{
		detector: detector,
	}
}

// GetCapabilities handles GET /api/hauler/capabilities
func (h *Handler) GetCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for refresh query parameter
	forceRefresh := r.URL.Query().Get("refresh") == "true"

	var caps *Capabilities
	var err error

	if forceRefresh {
		caps, err = h.detector.Refresh(r.Context())
	} else {
		caps, err = h.detector.Get(r.Context())
	}

	if err != nil {
		log.Printf("Error getting hauler capabilities: %v", err)
		http.Error(w, "Failed to get hauler capabilities", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(caps)
}

// RegisterRoutes registers the hauler routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/hauler/capabilities", h.GetCapabilities)
}
