package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/provider"
)

//go:embed static
var staticFS embed.FS

type dashApproval struct {
	ID       string
	Command  string
	Reason   string
	Response chan bool
}

// dashState is the dashboard session driver: same agent wiring as the REPL,
// messages over HTTP, observer events broadcast over SSE (D2, D51).
type dashState struct {
	mu          sync.Mutex
	cfgPath     string
	agent       *agent.Agent
	session     *sessionFile
	history     []provider.Message
	subs        map[chan agent.Event]struct{}
	busy        bool
	mode        guardrail.Mode
	workspace   string
	pending     *dashApproval
	approvalSeq uint64
	restored    int
}

type dashMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type dashSessionSummary struct {
	ID           string `json:"id"`
	Started      string `json:"started"`
	Updated      string `json:"updated,omitempty"`
	Title        string `json:"title"`
	Status       string `json:"status,omitempty"`
	MessageCount int    `json:"messageCount"`
}

type dashApprovalView struct {
	ID      string `json:"id"`
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

type dashSnapshot struct {
	SessionID string            `json:"sessionId"`
	Started   string            `json:"started"`
	Status    string            `json:"status"`
	Busy      bool              `json:"busy"`
	Workspace string            `json:"workspace"`
	Mode      string            `json:"mode"`
	Provider  string            `json:"provider"`
	Model     string            `json:"model"`
	Messages  []dashMessage     `json:"messages"`
	Approval  *dashApprovalView `json:"approval,omitempty"`
	Restored  int               `json:"restoredCount,omitempty"`
}

func newDashState(cfgPath string) (*dashState, error) {
	wd, _ := os.Getwd()
	d := &dashState{
		cfgPath:   cfgPath,
		subs:      map[chan agent.Event]struct{}{},
		mode:      configuredMode(cfgPath),
		workspace: filepath.Base(wd),
		session:   freshDashboardSession(),
	}
	if err := d.rebuild(); err != nil {
		return nil, err
	}
	return d, nil
}

func freshDashboardSession() *sessionFile {
	return &sessionFile{
		ID:      newSessionID(),
		Started: time.Now().Format(time.RFC3339),
		Status:  "ready",
	}
}

func (d *dashState) rebuild() error {
	a, err := buildAgent(d.cfgPath, nil, d.requestApproval)
	if err != nil {
		return err
	}
	a.Observer = d.broadcast
	d.agent = a
	return nil
}

func (d *dashState) broadcast(e agent.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for ch := range d.subs {
		select {
		case ch <- e:
		default: // slow client — drop; /api/state remains authoritative
		}
	}
}

func (d *dashState) requestApproval(ctx context.Context, command, reason string) (bool, error) {
	d.mu.Lock()
	if d.pending != nil {
		d.mu.Unlock()
		return false, fmt.Errorf("another approval is already pending")
	}
	d.approvalSeq++
	p := &dashApproval{
		ID:       fmt.Sprintf("approval-%d", d.approvalSeq),
		Command:  command,
		Reason:   reason,
		Response: make(chan bool, 1),
	}
	d.pending = p
	d.mu.Unlock()

	d.broadcast(agent.Event{Kind: "approval", ID: p.ID, Text: command, Detail: reason, Status: "pending"})

	var (
		allowed bool
		err     error
	)
	select {
	case allowed = <-p.Response:
	case <-ctx.Done():
		err = ctx.Err()
	}

	d.mu.Lock()
	if d.pending == p {
		d.pending = nil
	}
	d.mu.Unlock()

	status := "denied"
	if err != nil {
		status = "cancelled"
	} else if allowed {
		status = "approved"
	}
	d.broadcast(agent.Event{Kind: "approval", ID: p.ID, Text: command, Detail: reason, Status: status})
	return allowed, err
}

func (d *dashState) respondApproval(id string, allowed bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pending == nil || d.pending.ID != id {
		return fmt.Errorf("approval %q is no longer pending", id)
	}
	select {
	case d.pending.Response <- allowed:
		return nil
	default:
		return fmt.Errorf("approval %q already answered", id)
	}
}

// run executes one turn and persists it under one stable dashboard session ID.
func (d *dashState) run(ctx context.Context, text string) (string, string, error) {
	d.mu.Lock()
	if d.busy {
		d.mu.Unlock()
		return "", "", fmt.Errorf("busy: another turn is running")
	}
	d.busy = true
	d.history = append(d.history, provider.Message{Role: "user", Content: text})
	d.session.Messages = append([]provider.Message(nil), d.history...)
	d.session.Status = "working"
	d.session.Updated = time.Now().Format(time.RFC3339)
	before := *d.session
	before.Messages = append([]provider.Message(nil), d.session.Messages...)
	msgs := append([]provider.Message(nil), d.history...)
	a := d.agent
	d.mu.Unlock()
	_ = saveSession(&before)

	res := a.Run(ctx, msgs)

	d.mu.Lock()
	if res.Reply != "" {
		d.history = append(d.history, provider.Message{Role: "assistant", Content: res.Reply})
	}
	d.session.Messages = append([]provider.Message(nil), d.history...)
	d.session.Status = res.Status
	d.session.Updated = time.Now().Format(time.RFC3339)
	saved := *d.session
	saved.Messages = append([]provider.Message(nil), d.session.Messages...)
	d.busy = false
	d.mu.Unlock()
	_ = saveSession(&saved)
	return res.Status, res.Reply, nil
}

func (d *dashState) startNew() error {
	d.mu.Lock()
	if d.busy {
		d.mu.Unlock()
		return fmt.Errorf("busy: finish or stop the current turn first")
	}
	d.mu.Unlock()
	a, err := buildAgent(d.cfgPath, nil, d.requestApproval)
	if err != nil {
		return err
	}
	a.Observer = d.broadcast
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.busy {
		return fmt.Errorf("busy: finish or stop the current turn first")
	}
	d.agent = a
	d.session = freshDashboardSession()
	d.history = nil
	d.pending = nil
	d.restored = 0
	return nil
}

func (d *dashState) resume(id string) error {
	d.mu.Lock()
	if d.busy {
		d.mu.Unlock()
		return fmt.Errorf("busy: finish or stop the current turn first")
	}
	d.mu.Unlock()
	s, err := loadSession(id)
	if err != nil {
		return err
	}
	a, err := buildAgent(d.cfgPath, nil, d.requestApproval)
	if err != nil {
		return err
	}
	a.Observer = d.broadcast
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.busy {
		return fmt.Errorf("busy: finish or stop the current turn first")
	}
	d.agent = a
	d.session = s
	d.history = append([]provider.Message(nil), s.Messages...)
	d.pending = nil
	d.restored = len(d.history)
	return nil
}

func (d *dashState) snapshot() dashSnapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := dashSnapshot{
		Busy:      d.busy,
		Workspace: d.workspace,
		Mode:      string(d.mode),
		Status:    "ready",
		Restored:  d.restored,
	}
	if d.session != nil {
		s.SessionID = d.session.ID
		s.Started = d.session.Started
		s.Status = d.session.Status
	}
	if named, ok := d.agent.LLM.(interface {
		Name() string
		Model() string
	}); ok {
		s.Provider = named.Name()
		s.Model = named.Model()
	}
	for _, m := range d.history {
		if (m.Role == "user" || m.Role == "assistant") && strings.TrimSpace(m.Content) != "" {
			s.Messages = append(s.Messages, dashMessage{Role: m.Role, Content: m.Content})
		}
	}
	if d.pending != nil {
		s.Approval = &dashApprovalView{ID: d.pending.ID, Command: d.pending.Command, Reason: d.pending.Reason}
	}
	return s
}

func dashboardSessionSummaries() ([]dashSessionSummary, error) {
	sessions, err := listSessions()
	if err != nil {
		return nil, err
	}
	out := make([]dashSessionSummary, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, dashSessionSummary{
			ID:           s.ID,
			Started:      s.Started,
			Updated:      s.Updated,
			Title:        sessionTitle(s.Messages),
			Status:       s.Status,
			MessageCount: len(s.Messages),
		})
	}
	return out, nil
}

func sessionTitle(messages []provider.Message) string {
	for _, m := range messages {
		if m.Role != "user" {
			continue
		}
		title := strings.Join(strings.Fields(m.Content), " ")
		runes := []rune(title)
		if len(runes) > 72 {
			title = string(runes[:69]) + "…"
		}
		if title != "" {
			return title
		}
	}
	return "Untitled session"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// newDashboardMux wires the HTTP surface (static + API).
func newDashboardMux(d *dashState) (*http.ServeMux, error) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeAPIError(w, http.StatusMethodNotAllowed, fmt.Errorf("GET only"))
			return
		}
		writeJSON(w, http.StatusOK, d.snapshot())
	})
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeAPIError(w, http.StatusMethodNotAllowed, fmt.Errorf("GET only"))
			return
		}
		sessions, err := dashboardSessionSummaries()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, sessions)
	})
	mux.HandleFunc("/api/session/new", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAPIError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
			return
		}
		if err := d.startNew(); err != nil {
			writeAPIError(w, http.StatusConflict, err)
			return
		}
		writeJSON(w, http.StatusOK, d.snapshot())
	})
	mux.HandleFunc("/api/session/resume", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAPIError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
			return
		}
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if err := d.resume(body.ID); err != nil {
			writeAPIError(w, http.StatusConflict, err)
			return
		}
		writeJSON(w, http.StatusOK, d.snapshot())
	})
	mux.HandleFunc("/api/approval", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAPIError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
			return
		}
		var body struct {
			ID      string `json:"id"`
			Allowed bool   `json:"allowed"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		if err := d.respondApproval(body.ID, body.Allowed); err != nil {
			writeAPIError(w, http.StatusConflict, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeAPIError(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
			return
		}
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		body.Text = strings.TrimSpace(body.Text)
		if body.Text == "" {
			writeAPIError(w, http.StatusBadRequest, fmt.Errorf("task cannot be empty"))
			return
		}
		status, reply, err := d.run(r.Context(), body.Text)
		if err != nil {
			writeAPIError(w, http.StatusConflict, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": status, "reply": reply})
	})
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
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
		fmt.Fprint(w, ": connected\n\n")
		flusher.Flush()
		for {
			select {
			case e := <-ch:
				data, _ := json.Marshal(e)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
	return mux, nil
}

// dashboard serves the embedded web UI (localhost only).
func dashboard(out io.Writer, cfgPath string) error {
	d, err := newDashState(cfgPath)
	if err != nil {
		return err
	}
	mux, err := newDashboardMux(d)
	if err != nil {
		return err
	}
	addr := "127.0.0.1:8787"
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if strings.Contains(err.Error(), "in use") {
			return fmt.Errorf("port 8787 is already in use — is another fender dashboard running? (kill it with: pkill -f 'fender dashboard')")
		}
		return err
	}
	fmt.Fprintf(out, "fender dashboard at http://%s (ctrl-c to stop)\n", addr)
	return http.Serve(ln, mux)
}
