package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/H4fizWasabie/fender/internal/agent"
	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/skills"
)

// repl is the interactive loop (D26): slash commands + Agent.Run with
// observer rendering. History lives in memory for the session (D9).
func repl(out, errOut io.Writer, in *bufio.Reader, cfgPath string) error {
	fmt.Fprintf(out, "fender %s — type /help for commands\n", version)

	state := &replState{cfgPath: cfgPath, mode: guardrail.Balanced}
	state.rebuild = func() error {
		approver := func(cmd, reason string) (bool, error) {
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
			switch e.Kind {
			case "delta":
				fmt.Fprint(out, e.Text)
			case "tool":
				status := e.Status
				if status == "" {
					status = "ok"
				}
				fmt.Fprintf(out, "\n  [tool %s: %s]\n", e.Text, status)
			case "done":
				fmt.Fprintf(out, "\n<%s>\n", e.Status)
			}
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
		fmt.Fprint(out, "> ")
		line, err := in.ReadString('\n')
		if err != nil { // EOF
			fmt.Fprintln(out)
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
		if res.Status == "complete" || res.Status == "blocked" {
			state.history = append(state.history, provider.Message{Role: "assistant", Content: res.Reply})
		}
	}
}

// replState carries the REPL's mutable state between turns.
type replState struct {
	cfgPath string
	mode    guardrail.Mode
	agent   *agent.Agent
	skills  *skills.Registry
	history []provider.Message
	rebuild func() error
}

// slash handles one slash command; returns quit=true for /quit.
func slash(out io.Writer, text string, st *replState) (bool, error) {
	parts := strings.Fields(text)
	switch parts[0] {
	case "/quit":
		return true, nil
	case "/help":
		fmt.Fprintln(out, "commands: /quit /model <provider> /mode <strict|balanced|yolo> /skills /help")
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
		st.agent.LLM = c
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
