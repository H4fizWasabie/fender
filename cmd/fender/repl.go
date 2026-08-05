package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/skills"
)

// repl is the interactive loop (D26): slash commands + Agent.Run with
// observer rendering. History lives in memory for the session (D9).
func repl(out, errOut io.Writer, in *bufio.Reader, cfgPath, resumeID string) error {
	fmt.Fprintf(out, "fender %s — type /help for commands\n", version)

	state := &replState{cfgPath: cfgPath, mode: guardrail.Balanced}
	if resumeID != "" {
		var prev *sessionFile
		var err error
		if resumeID == "latest" {
			prev, err = loadLatestSession()
		} else {
			prev, err = loadSession(resumeID)
		}
		if err != nil {
			return fmt.Errorf("resume session: %w", err)
		}
		if prev == nil {
			return fmt.Errorf("resume session: no saved sessions")
		}
		state.history = prev.Messages
		fmt.Fprintf(out, "resumed session %s (%d messages)\n", prev.ID, len(prev.Messages))
	}
	state.session = &sessionFile{ID: newSessionID(), Started: time.Now().Format(time.RFC3339)}
	var streamed bool // any delta shown this run (reply may duplicate)
	state.rebuild = func() error {
		approver := func(_ context.Context, cmd, reason string) (bool, error) {
			fmt.Fprintf(out, "\n  [approval] %s\n  %s [y/N] ", reason, cmd)
			line, err := in.ReadString('\n')
			if err != nil {
				return false, err
			}
			return strings.TrimSpace(strings.ToLower(line)) == "y", nil
		}
		a, err := buildAgent(cfgPath, &state.mode, approver)
		if err != nil {
			return err
		}
		a.Observer = func(e agent.Event) {
			if e.Kind == "delta" && e.Text != "" {
				streamed = true
			}
			renderEvent(out, e, state.thinking != "")
		}
		state.agent = a
		state.skills = a.Skills
		return nil
	}
	if err := state.rebuild(); err != nil {
		fmt.Fprintf(errOut, "warning: %v\n", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	for {
		model := "?"
		if state.agent != nil {
			if c, ok := state.agent.LLM.(interface {
				Name() string
				Model() string
			}); ok {
				model = c.Name() + "/" + c.Model()
			}
		}
		fmt.Fprintf(out, "\x1b[2m[%s %s]\x1b[0m > ", state.mode, model)
		line, err := in.ReadString('\n')
		if err != nil { // EOF
			fmt.Fprintln(out)
			state.save()
			state.distill()
			return nil
		}
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if strings.HasPrefix(text, "/") {
			quit, err := slash(out, text, state)
			if err != nil {
				fmt.Fprintf(out, "error: %v\n", err)
			}
			if quit {
				state.save()
				state.distill()
				return nil
			}
			continue
		}
		if state.agent == nil {
			fmt.Fprintln(out, "error: agent not built (config problem above)")
			continue
		}
		state.history = append(state.history, provider.Message{Role: "user", Content: text})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			select {
			case <-sig:
				cancel()
			case <-done:
			}
		}()
		res := state.agent.Run(ctx, state.history)
		close(done)
		cancel()
		if !streamed && res.Reply != "" {
			fmt.Fprintln(out, res.Reply) // answer arrived via complete_task args, not deltas
		}
		streamed = false
		if res.Status == "complete" || res.Status == "blocked" {
			state.history = append(state.history, provider.Message{Role: "assistant", Content: res.Reply})
		}
		state.save() // persist after every turn (D41)
	}
}

// replState carries the REPL's mutable state between turns.
type replState struct {
	cfgPath  string
	mode     guardrail.Mode
	agent    *agent.Agent
	skills   *skills.Registry
	history  []provider.Message
	thinking string // "" = hidden (off); non-empty = show dimmed
	session  *sessionFile
	rebuild  func() error
}

// distill fires background consolidation at session end (D43).
func (st *replState) distill() {
	if st.session == nil || st.agent == nil {
		return
	}
	if llm, ok := st.agent.LLM.(agentLLM); ok {
		go func() {
			consolidateSession(st.session, llm, ".")
		}()
	}
}

// save persists the current history (D41). Failures are non-fatal.
func (st *replState) save() {
	if st.session == nil {
		return
	}
	st.session.Messages = st.history
	saveSession(st.session) // ignore errors — persistence is best-effort
}

// slash handles one slash command; returns quit=true for /quit.
func slash(out io.Writer, text string, st *replState) (bool, error) {
	parts := strings.Fields(text)
	switch parts[0] {
	case "/quit":
		return true, nil
	case "/help":
		fmt.Fprintln(out, "commands: /quit /model <provider> /mode <strict|balanced|yolo> /thinking <off|low|medium|high> /skills /help")
		return false, nil
	case "/model":
		if len(parts) < 2 {
			return false, fmt.Errorf("usage: /model <provider>")
		}
		var (
			reg *provider.Registry
			err error
		)
		if st.cfgPath != "" {
			reg, err = provider.Load(st.cfgPath)
		} else {
			reg, err = provider.LoadDefault()
		}
		if err != nil {
			return false, err
		}
		c, ok := reg.Client(parts[1])
		if !ok {
			return false, fmt.Errorf("unknown provider %q", parts[1])
		}
		if st.agent == nil {
			return false, fmt.Errorf("no agent built yet")
		}
		configured, err := reg.WithFallback(c)
		if err != nil {
			return false, err
		}
		st.agent.LLM = configured
		fmt.Fprintf(out, "model -> %s\n", parts[1])
		return false, nil
	case "/mode":
		if len(parts) < 2 {
			return false, fmt.Errorf("usage: /mode <strict|balanced|yolo>")
		}
		m := guardrail.Mode(parts[1])
		if !validMode(m) {
			return false, fmt.Errorf("invalid mode %q (strict|balanced|yolo)", parts[1])
		}
		st.mode = m
		if err := st.rebuild(); err != nil {
			return false, err
		}
		fmt.Fprintf(out, "mode -> %s\n", m)
		return false, nil
	case "/thinking":
		if len(parts) < 2 {
			return false, fmt.Errorf("usage: /thinking <off|low|medium|high>")
		}
		level := parts[1]
		if level != "off" && level != "low" && level != "medium" && level != "high" {
			return false, fmt.Errorf("invalid level %q (off|low|medium|high)", level)
		}
		c, ok := st.agent.LLM.(interface{ SetThinking(string) error })
		if !ok {
			return false, fmt.Errorf("current LLM has no thinking control")
		}
		if err := c.SetThinking(level); err != nil {
			return false, err
		}
		if level == "off" {
			level = ""
		}
		st.thinking = level
		fmt.Fprintf(out, "thinking -> %s\n", parts[1])
		return false, nil
	case "/skills":
		if st.skills == nil {
			return false, fmt.Errorf("skills registry unavailable")
		}
		fmt.Fprint(out, st.skills.Descriptions())
		return false, nil
	default:
		return false, fmt.Errorf("unknown command %q (try /help)", parts[0])
	}
}

func validMode(m guardrail.Mode) bool {
	switch m {
	case guardrail.Strict, guardrail.Balanced, guardrail.Yolo:
		return true
	}
	return false
}

// renderEvent draws one observer event (ticket-08 renderer seam). Thinking
// deltas render dimmed only when showThinking is set (D40). Ephemeral child
// events carry a source prefix so child work remains observable (D50).
func renderEvent(out io.Writer, e agent.Event, showThinking bool) {
	prefix := ""
	if e.Source != "" {
		prefix = "\x1b[35m[" + e.Source + "]\x1b[0m " // magenta tag
	}
	switch e.Kind {
	case "delta":
		fmt.Fprint(out, prefix, e.Text)
	case "thinking":
		if showThinking {
			fmt.Fprintf(out, "\x1b[2m%s%s\x1b[0m", prefix, e.Text)
		}
	case "tool":
		status := e.Status
		if status == "" {
			status = "ok"
		}
		fmt.Fprintf(out, "\n  %s[tool %s: %s]\n", prefix, e.Text, status)
	case "done":
		if e.Source != "" {
			return // child completion is reported via the delegate tool result
		}
		color := ""
		switch e.Status {
		case "complete":
			color = "\x1b[32m" // green
		case "blocked":
			color = "\x1b[33m" // yellow
		case "error", "stalled", "cancelled":
			color = "\x1b[31m" // red
		}
		fmt.Fprintf(out, "\n%s<%s>\x1b[0m\n", color, e.Status)
	}
}
