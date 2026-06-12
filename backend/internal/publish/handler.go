package publish

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/hauler-ui/hauler-ui/backend/internal/hauls"
)

// Handler exposes the publish API and path-routed file serving.
type Handler struct {
	mgr   *Manager
	hauls *hauls.Service
}

// NewHandler creates a publish HTTP handler.
func NewHandler(mgr *Manager, haulSvc *hauls.Service) *Handler {
	return &Handler{mgr: mgr, hauls: haulSvc}
}

// RegisterRoutes wires publish endpoints and the /h/ file routes into the mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/publish", h.handleList)    // GET routes + config
	mux.HandleFunc("/api/publish/tls", h.handleTLS) // GET/POST/DELETE registry cert
	mux.HandleFunc("/api/publish/", h.handleByID)   // POST/DELETE /api/publish/{haulId}
	mux.HandleFunc("/h/", h.handleFiles)            // GET /h/{slug}[/{name}]
}

type tlsRequest struct {
	CertPem string `json:"certPem"`
	KeyPem  string `json:"keyPem"`
}

// handleTLS manages the registry listener's TLS certificate.
//
//	GET    -> current cert status
//	POST   -> load a provided cert/key (PEM); takes effect on next handshake
//	DELETE -> drop the provided cert and revert to self-signed
func (h *Handler) handleTLS(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.mgr.TLSStatus())
	case http.MethodPost:
		var req tlsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.CertPem == "" || req.KeyPem == "" {
			http.Error(w, "certPem and keyPem are required", http.StatusBadRequest)
			return
		}
		if err := h.mgr.SetProvidedTLS([]byte(req.CertPem), []byte(req.KeyPem)); err != nil {
			http.Error(w, "failed to load certificate: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Certificate loaded. Restart the server if the registry was started without TLS.",
			"status":  h.mgr.TLSStatus(),
		})
	case http.MethodDelete:
		if err := h.mgr.ClearProvidedTLS(); err != nil {
			http.Error(w, "failed to clear certificate: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"message": "Reverted to self-signed certificate", "status": h.mgr.TLSStatus()})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// handleList returns the routes table plus the registry domain/port config so
// the UI can render copy-paste client snippets.
func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"routes":         h.mgr.List(r.Context()),
		"registryDomain": os.Getenv("HAULER_UI_REGISTRY_DOMAIN"),
		"registryPort":   registryListenPort(),
	})
}

type publishRequest struct {
	Hostname string `json:"hostname"`
}

// handleByID publishes (POST) or unpublishes (DELETE) a haul.
func (h *Handler) handleByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/publish/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid haul id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req publishRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		p, err := h.mgr.Publish(r.Context(), id, req.Hostname)
		if err != nil {
			http.Error(w, "Failed to publish: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"haulId":   p.HaulID,
			"hostname": p.Hostname,
			"port":     p.Port,
			"message":  "Haul published",
		})
	case http.MethodDelete:
		if err := h.mgr.Unpublish(r.Context(), id); err != nil {
			http.Error(w, "Failed to unpublish: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"message": "Haul unpublished"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFiles serves a haul's files directly from its store:
//
//	GET /h/{slug}/         -> JSON listing
//	GET /h/{slug}/{name}   -> stream the file
func (h *Handler) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/h/")
	slug := rest
	name := ""
	if i := strings.IndexByte(rest, '/'); i != -1 {
		slug = rest[:i]
		name = rest[i+1:]
	}
	if slug == "" {
		http.Error(w, "haul slug required", http.StatusBadRequest)
		return
	}

	haul, err := h.hauls.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "haul not found", http.StatusNotFound)
		return
	}

	if name == "" {
		serveFileList(w, haul.StoreDir)
		return
	}
	serveFile(w, r, haul.StoreDir, name)
}

// registryListenPort returns the configured host-routed registry port.
func registryListenPort() int {
	if v := os.Getenv("HAULER_UI_REGISTRY_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			return p
		}
	}
	return 5000
}
