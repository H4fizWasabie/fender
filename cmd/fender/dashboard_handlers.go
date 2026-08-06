package main

import (
	"fmt"
	"net/http"
	"strings"
)

func (d *dashState) handleState(w http.ResponseWriter, r *http.Request) {
	if requireMethod(w, r, http.MethodGet) {
		writeJSON(w, http.StatusOK, d.snapshot())
	}
}

func (d *dashState) handleSessions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	sessions, err := dashboardSessionSummaries()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (d *dashState) handleNewSession(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if err := d.startNew(); err != nil {
		writeAPIError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, d.snapshot())
}

func (d *dashState) handleResumeSession(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(w, r, 1<<20, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if err := d.resume(body.ID); err != nil {
		writeAPIError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, d.snapshot())
}

func (d *dashState) handleApproval(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct {
		ID      string `json:"id"`
		Allowed bool   `json:"allowed"`
	}
	if err := decodeJSON(w, r, 1<<20, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if err := d.respondApproval(body.ID, body.Allowed); err != nil {
		writeAPIError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (d *dashState) handleMessage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(w, r, 2<<20, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	body.Text = strings.TrimSpace(body.Text)
	if body.Text == "" {
		writeAPIError(w, http.StatusBadRequest, fmt.Errorf("task cannot be empty"))
		return
	}
	// D59: "/skill task..." injects the skill body into the turn
	if composed, isSkill, err := skillTask(d.agent, body.Text); isSkill {
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		body.Text = composed
	}
	d.mu.Lock()
	busy := d.busy
	d.mu.Unlock()
	if busy {
		// D58: a message while working is a STEER — delivered mid-run at the
		// next safe boundary (pi-style interrupt-and-inject).
		if d.agent != nil {
			d.agent.Steer(body.Text)
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "steered"})
		return
	}
	status, reply, err := d.run(r.Context(), body.Text)
	if err != nil {
		writeAPIError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status, "reply": reply})
}
