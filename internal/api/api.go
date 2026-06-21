// Package api exposes the local REST API consumed by the web UI and centralizes the logic for "whenever something changes, recalculate the hosts file".
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ezerevello/focusguard/internal/presets"
	"github.com/ezerevello/focusguard/internal/store"
	"github.com/ezerevello/focusguard/internal/sysutil"
	"github.com/ezerevello/focusguard/internal/hostsfile"
)

const Version = "0.1.0"

// Engine binds the store with the hosts file and exposes the HTTP router.
type Engine struct {
	st *store.Store
}

func New(st *store.Store) *Engine {
	return &Engine{st: st}
}

// Router builds the http.ServeMux with all the API routes.
// Requires Go 1.22+ for standard library routing with methods and {wildcards}.
func (e *Engine) Router() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/status", e.handleStatus)
	mux.HandleFunc("GET /api/sites", e.handleListSites)
	mux.HandleFunc("POST /api/sites", e.handleAddSite)
	mux.HandleFunc("DELETE /api/sites/{id}", e.handleDeleteSite)
	mux.HandleFunc("POST /api/sites/{id}/toggle", e.handleToggleSite)
	mux.HandleFunc("GET /api/presets", e.handleListPresets)
	mux.HandleFunc("POST /api/presets/{key}", e.handleAddPreset)
	mux.HandleFunc("GET /api/session", e.handleGetSession)
	mux.HandleFunc("POST /api/session/start", e.handleStartSession)

	return mux
}

// applyHosts recalculates the list of blocked domains and writes it to the hosts file. It is called after any state changes.
func (e *Engine) applyHosts() error {
	domains := e.st.BlockedDomains()
	if err := hostsfile.Apply(domains); err != nil {
		log.Printf("error applying hosts file: %v", err)
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// ---- handlers ----

func (e *Engine) handleStatus(w http.ResponseWriter, r *http.Request) {
	e.st.ExpireSessionIfDue()
	blocked, _ := hostsfile.CurrentlyBlocked()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":         Version,
		"os":              sysutil.OS(),
		  "hosts_path":      hostsfile.Path(),
		  "elevated":        sysutil.IsElevated(),
		  "sites_count":     len(e.st.Sites()),
		  "domains_blocked": len(blocked),
		  "session":         e.st.Session(),
	})
}

func (e *Engine) handleListSites(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, e.st.Sites())
}

type addSiteReq struct {
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
}

func (e *Engine) handleAddSite(w http.ResponseWriter, r *http.Request) {
	var req addSiteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Domains) == 0 {
		writeErr(w, http.StatusBadRequest, errStr("name and at least one domain are required"))
		return
	}
	site, err := e.st.AddSite(req.Name, req.Domains, "")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, site)
}

func (e *Engine) handleDeleteSite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := e.st.RemoveSite(id); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := e.applyHosts(); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type toggleReq struct {
	Enabled bool `json:"enabled"`
}

func (e *Engine) handleToggleSite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req toggleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := e.st.SetEnabled(id, req.Enabled); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	if err := e.applyHosts(); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (e *Engine) handleListPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, presets.All())
}

func (e *Engine) handleAddPreset(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	for _, p := range presets.All() {
		if p.Key == key {
			site, err := e.st.AddSite(p.Name, p.Domains, p.Key)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusCreated, site)
			return
		}
	}
	writeErr(w, http.StatusNotFound, errStr("preset not found"))
}

func (e *Engine) handleGetSession(w http.ResponseWriter, r *http.Request) {
	e.st.ExpireSessionIfDue()
	writeJSON(w, http.StatusOK, e.st.Session())
}

type startSessionReq struct {
	SiteIDs []string `json:"site_ids"`
	Minutes int      `json:"minutes"`
	Label   string   `json:"label"`
}

func (e *Engine) handleStartSession(w http.ResponseWriter, r *http.Request) {
	var req startSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Minutes <= 0 || len(req.SiteIDs) == 0 {
		writeErr(w, http.StatusBadRequest, errStr("minutes and site_ids are required"))
		return
	}
	sess, err := e.st.StartSession(req.SiteIDs, time.Duration(req.Minutes)*time.Minute, req.Label)
	if err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	if err := e.applyHosts(); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

type errStr string

func (e errStr) Error() string { return string(e) }
