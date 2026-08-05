package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/provider"
)

func TestDashboardServe(t *testing.T) {
	cfg := writeConfig(t, `
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := newDashboardMux(d)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "What should Fender do?") {
		t.Fatalf("index.html missing workbench heading: %.200s", body)
	}
}

// dashFakeLLM completes instantly via the completion protocol.
type dashFakeLLM struct{}

func (d *dashFakeLLM) Chat(ctx context.Context, req provider.Request) (*provider.Response, error) {
	return &provider.Response{Choices: []provider.Choice{{Message: provider.Message{
		Role: "assistant",
		ToolCalls: []provider.ToolCall{{
			ID: "call_c", Type: "function",
			Function: provider.ToolFunction{Name: "complete_task", Arguments: `{"status":"complete","reply":"hi there"}`},
		}},
	}}}}, nil
}

type dashFinalSaveFailureLLM struct{}

func (d *dashFinalSaveFailureLLM) Chat(ctx context.Context, req provider.Request) (*provider.Response, error) {
	if err := os.RemoveAll(".fender/sessions"); err != nil {
		return nil, err
	}
	if err := os.WriteFile(".fender/sessions", []byte("not a directory"), 0600); err != nil {
		return nil, err
	}
	return (&dashFakeLLM{}).Chat(ctx, req)
}

func TestDashStateRun(t *testing.T) {
	chdir(t, t.TempDir())
	cfg := writeConfig(t, `
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	d.agent.LLM = &dashFakeLLM{}
	sessionID := d.session.ID

	status, reply, err := d.run(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if status != "complete" || reply != "hi there" {
		t.Fatalf("status=%q reply=%q", status, reply)
	}
	if len(d.history) != 2 {
		t.Fatalf("history = %+v", d.history)
	}
	if d.session.ID != sessionID {
		t.Fatalf("dashboard session changed ID across one turn: %q -> %q", sessionID, d.session.ID)
	}
	saved, err := loadSession(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "complete" || len(saved.Messages) != 2 {
		t.Fatalf("saved session = %+v", saved)
	}

	// broadcast reaches subscribers
	ch := make(chan agent.Event, 8)
	d.mu.Lock()
	d.subs[ch] = struct{}{}
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.subs, ch); d.mu.Unlock() }()
	d.broadcast(agent.Event{Kind: "tool", Text: "shell", Status: "ok"})
	select {
	case e := <-ch:
		if e.Kind != "tool" {
			t.Fatalf("event = %+v", e)
		}
	default:
		t.Fatal("subscriber missed broadcast")
	}
}

func TestDashboardSessionAPIs(t *testing.T) {
	chdir(t, t.TempDir())
	seed := &sessionFile{
		ID:      "20260805-090000",
		Started: "2026-08-05T09:00:00Z",
		Status:  "complete",
		Messages: []provider.Message{
			{Role: "user", Content: "repair the parser"},
			{Role: "assistant", Content: "parser repaired"},
		},
	}
	if err := saveSession(seed); err != nil {
		t.Fatal(err)
	}
	cfg := writeConfig(t, `
mode = "strict"
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := newDashboardMux(d)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var initial dashSnapshot
	getJSON(t, srv.URL+"/api/state", &initial)
	if initial.SessionID == seed.ID || len(initial.Messages) != 0 || initial.Mode != "strict" {
		t.Fatalf("dashboard did not start fresh: %+v", initial)
	}

	var sessions []dashSessionSummary
	getJSON(t, srv.URL+"/api/sessions", &sessions)
	if len(sessions) != 1 || sessions[0].Title != "repair the parser" {
		t.Fatalf("session summaries = %+v", sessions)
	}

	var resumed dashSnapshot
	postJSON(t, srv.URL+"/api/session/resume", map[string]string{"id": seed.ID}, &resumed)
	if resumed.SessionID != seed.ID || len(resumed.Messages) != 2 {
		t.Fatalf("resumed state = %+v", resumed)
	}
	if resumed.Restored != 2 {
		t.Fatalf("restored boundary = %d, want 2", resumed.Restored)
	}

	var fresh dashSnapshot
	postJSON(t, srv.URL+"/api/session/new", map[string]any{}, &fresh)
	if fresh.SessionID == seed.ID || len(fresh.Messages) != 0 || fresh.Status != "ready" || fresh.Restored != 0 {
		t.Fatalf("fresh state = %+v", fresh)
	}
}

func TestDashboardApprovalLifecycle(t *testing.T) {
	chdir(t, t.TempDir())
	cfg := writeConfig(t, `
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan agent.Event, 4)
	d.mu.Lock()
	d.subs[events] = struct{}{}
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.subs, events)
		d.mu.Unlock()
	}()

	result := make(chan bool, 1)
	go func() {
		allowed, _ := d.requestApproval(context.Background(), "git reset --hard", "irreversible git")
		result <- allowed
	}()
	pending := <-events
	if pending.Kind != "approval" || pending.Status != "pending" || pending.ID == "" {
		t.Fatalf("pending event = %+v", pending)
	}
	if err := d.respondApproval(pending.ID, true); err != nil {
		t.Fatal(err)
	}
	if !<-result {
		t.Fatal("approval response was not delivered")
	}
	resolved := <-events
	if resolved.Status != "approved" {
		t.Fatalf("resolved event = %+v", resolved)
	}
}

func TestDashboardSnapshotReportsTerminalTruth(t *testing.T) {
	chdir(t, t.TempDir())
	cfg := writeConfig(t, `
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	mux, err := newDashboardMux(d)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d.session.Status = "working"
	var working map[string]any
	getJSON(t, srv.URL+"/api/state", &working)
	if terminal, ok := working["terminal"].(bool); !ok || terminal {
		t.Fatalf("working snapshot terminal = %#v, want false", working["terminal"])
	}

	d.session.Status = "complete"
	var complete map[string]any
	getJSON(t, srv.URL+"/api/state", &complete)
	if terminal, ok := complete["terminal"].(bool); !ok || !terminal {
		t.Fatalf("complete snapshot terminal = %#v, want true", complete["terminal"])
	}
}

func TestDashboardPersistsEvidenceEvents(t *testing.T) {
	chdir(t, t.TempDir())
	cfg := writeConfig(t, `
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveSession(d.session); err != nil {
		t.Fatal(err)
	}
	d.broadcast(agent.Event{Kind: "tool", Text: "shell", Status: "ok", Detail: "tests passed"})

	data, err := os.ReadFile(".fender/sessions/" + d.session.ID + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	events, ok := saved["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("persisted events = %#v, want one event", saved["events"])
	}
}

func TestDashboardDoesNotPersistDoneBeforeTerminalState(t *testing.T) {
	chdir(t, t.TempDir())
	cfg := writeConfig(t, `
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	d.session.Status = "working"
	if err := saveSession(d.session); err != nil {
		t.Fatal(err)
	}
	d.broadcast(agent.Event{Kind: "done", Text: "finished", Status: "complete"})

	saved, err := loadSession(d.session.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range saved.Events {
		if event.Kind == "done" {
			t.Fatalf("done event persisted before terminal session state: %+v", saved)
		}
	}
}

func TestDashboardRunReportsInitialPersistenceFailure(t *testing.T) {
	chdir(t, t.TempDir())
	cfg := writeConfig(t, `
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	d.agent.LLM = &dashFakeLLM{}
	if err := os.MkdirAll(".fender", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".fender/sessions", []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := d.run(context.Background(), "hello"); err == nil {
		t.Fatal("run succeeded even though its working session could not be persisted")
	}
}

func TestDashboardRunReportsFinalPersistenceFailure(t *testing.T) {
	chdir(t, t.TempDir())
	cfg := writeConfig(t, `
[providers.mock]
base_url = "http://localhost:1/v1"
api_key = "k"
models = ["m1"]
default_model = "m1"
`)
	d, err := newDashState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	d.agent.LLM = &dashFinalSaveFailureLLM{}

	status, reply, err := d.run(context.Background(), "hello")
	if err == nil {
		t.Fatal("run hid its final persistence failure")
	}
	if status != "complete" || reply != "hi there" {
		t.Fatalf("runtime result lost on save failure: status=%q reply=%q", status, reply)
	}
	if d.snapshot().PersistenceError == "" {
		t.Fatal("snapshot did not expose the persistence failure")
	}
}

func getJSON(t *testing.T, url string, target any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: %s: %s", url, resp.Status, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func postJSON(t *testing.T, url string, body, target any) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s: %s: %s", url, resp.Status, responseBody)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}
