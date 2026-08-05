package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/H4fizWasabie/fender/internal/agent"
)

func durableEvent(e agent.Event) bool {
	return e.Kind == "tool" || e.Kind == "approval" || e.Kind == "done"
}

func (d *dashState) broadcast(e agent.Event) {
	var saved *sessionFile
	d.mu.Lock()
	if durableEvent(e) && d.session != nil {
		d.events = append(d.events, e)
		d.session.Events = append([]agent.Event(nil), d.events...)
		// A done event becomes durable only with finishTurn's terminal status.
		// Persisting it while status is still working can fabricate completion
		// after a crash or final-save failure.
		if e.Kind != "done" {
			clone := cloneSession(d.session)
			saved = &clone
		}
	}
	for ch := range d.subs {
		select {
		case ch <- e:
		default:
		}
	}
	d.mu.Unlock()

	if saved != nil {
		if err := saveSession(saved); err != nil {
			d.recordPersistenceError(fmt.Errorf("save observer evidence: %w", err))
			slog.Error("dashboard evidence persistence failed", "session", saved.ID, "error", err)
		}
	}
}

func (d *dashState) serveEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no flush", http.StatusInternalServerError)
		return
	}
	ch := make(chan agent.Event, 64)
	d.mu.Lock()
	d.subs[ch] = struct{}{}
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.subs, ch)
		d.mu.Unlock()
	}()
	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()
	for {
		select {
		case e := <-ch:
			data, err := json.Marshal(e)
			if err != nil {
				slog.Error("marshal dashboard event", "error", err)
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
