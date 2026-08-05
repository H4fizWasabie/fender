package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write dashboard JSON", "error", err)
	}
}

func writeAPIError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	writeAPIError(w, http.StatusMethodNotAllowed, fmt.Errorf("%s only", method))
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, target any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, limit)).Decode(target)
}

func newDashboardMux(d *dashState) (*http.ServeMux, error) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/state", d.handleState)
	mux.HandleFunc("/api/sessions", d.handleSessions)
	mux.HandleFunc("/api/session/new", d.handleNewSession)
	mux.HandleFunc("/api/session/resume", d.handleResumeSession)
	mux.HandleFunc("/api/approval", d.handleApproval)
	mux.HandleFunc("/api/message", d.handleMessage)
	mux.HandleFunc("/api/events", d.serveEvents)
	return mux, nil
}
