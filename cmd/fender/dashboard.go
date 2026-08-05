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
	"strings"
	"sync"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/provider"
)

//go:embed static
var staticFS embed.FS

// dashState is the dashboard session driver: same agent wiring as the REPL,
// messages over HTTP, observer events broadcast over SSE (D2 seam).
type dashState struct {
	mu      sync.Mutex
	cfgPath string
	agent   *agent.Agent
	history []provider.Message
	subs    map[chan agent.Event]struct{}
	busy    bool
}

func newDashState(cfgPath string) (*dashState, error) {
	d := &dashState{cfgPath: cfgPath, subs: map[chan agent.Event]struct{}{}}
	if err := d.rebuild(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *dashState) rebuild() error {
	a, err := buildAgent(d.cfgPath, nil, nil)
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
		default: // slow client — drop
		}
	}
}

// run executes one turn and returns the final reply.
func (d *dashState) run(ctx context.Context, text string) (string, string, error) {
	d.mu.Lock()
	if d.busy {
		d.mu.Unlock()
		return "", "", fmt.Errorf("busy: another turn is running")
	}
	d.busy = true
	d.mu.Unlock()
	defer func() { d.mu.Lock(); d.busy = false; d.mu.Unlock() }()

	d.mu.Lock()
	d.history = append(d.history, provider.Message{Role: "user", Content: text})
	msgs := append([]provider.Message(nil), d.history...)
	d.mu.Unlock()

	res := d.agent.Run(ctx, msgs)
	if res.Status == "complete" || res.Status == "blocked" {
		d.mu.Lock()
		d.history = append(d.history, provider.Message{Role: "assistant", Content: res.Reply})
		saveSession(&sessionFile{ID: newSessionID(), Started: "dashboard", Messages: d.history})
		d.mu.Unlock()
	}
	return res.Status, res.Reply, nil
}

// newDashboardMux wires the HTTP surface (static + API).
func newDashboardMux(d *dashState) (*http.ServeMux, error) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		status, reply, err := d.run(r.Context(), body.Text)
		if err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": status, "reply": reply})
	})
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flush", 500)
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
