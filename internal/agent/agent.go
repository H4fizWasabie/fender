// Package agent implements D13: ONE loop. Agent{LLM, Tools} runs the
// canonical loop (prompt -> LLM -> tool call -> execute -> result -> repeat).
// A child agent is the same type with fresh ephemeral state (delegate tool).
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/H4fizWasabie/fender/internal/memory"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/skills"
	"github.com/H4fizWasabie/fender/internal/tools"
)

// LLM is what the loop needs from the provider layer. *provider.Client
// satisfies it (it fills the model when Request.Model is empty); tests use
// a scripted fake — the second implementation that justifies the interface.
type LLM interface {
	Chat(ctx context.Context, req provider.Request) (*provider.Response, error)
}

// Event is one observable loop event (the renderer seam, ticket-08 spec §3.1).
// JSON tags are load-bearing: the dashboard SSE (ticket 12) marshals events
// and the browser switches on lowercase keys.
type Event struct {
	Kind   string `json:"kind"`             // "delta" | "thinking" | "tool" | "approval" | "done"
	Text   string `json:"text"`             // delta text / tool description / final reply
	Status string `json:"status"`           // tool status ("ok"|"error"|"cached") or result status
	Source string `json:"source,omitempty"` // "" = main agent; "child" = ephemeral child (D50)
	ID     string `json:"id,omitempty"`     // pending interaction identity (D51 approval holds)
	Detail string `json:"detail,omitempty"` // user-facing reason or supporting runtime detail
}

// Streamer is the optional streaming capability of an LLM (spec §3.2).
// onThinking receives reasoning deltas when the provider streams them (D40).
type Streamer interface {
	StreamChat(ctx context.Context, req provider.Request, onDelta func(string), onThinking ...func(string)) (*provider.Response, error)
}

const (
	defaultMaxIter    = 30
	defaultMaxSubIter = 6
	maxEventDetail    = 8_000
	orientationPrompt = `(orientation turn — harness-enforced) Stop and orient before acting again. Reply with exactly four points: 1) what you know, 2) what is uncertain, 3) your hypothesis for the failures so far, 4) the single next distinct action. Do not repeat a failed or already-executed call.`
)

// Agent is one loop: model + tools + discipline. The same type runs the
// main agent and every ephemeral child (D13, D50).
type Agent struct {
	LLM        LLM
	System     string
	MaxIter    int              // 0 -> defaultMaxIter
	MaxSubIter int              // 0 -> defaultMaxSubIter
	Meter      *provider.Meter // real token accounting (D56); nil-safe
	Observer   func(Event)      // renderer seam (ticket 08); nil-safe
	steerMu    sync.Mutex
	steer      string        // pending steering message (D58), latest-wins
	steerCh    chan struct{} // signal: a steer arrived (buffered 1)
	delivered  []string      // steers actually injected into the conversation
	Mem        *memory.Memory   // D39 ICM memory workspace; nil = ticket-04 behavior
	Skills     *skills.Registry // D27 skills; nil = ticket-05 behavior
	registry   *tools.Registry
}

// NewAgent wires llm to reg and registers the delegate tool (D13).
func NewAgent(llm LLM, reg *tools.Registry) *Agent {
	a := &Agent{LLM: llm, registry: reg, steerCh: make(chan struct{}, 1)}
	a.registry.Add(a.delegateTool())
	a.registry.Add(a.loadSkillTool())
	return a
}

// subIter is the effective subagent iteration cap.
func (a *Agent) subIter() int {
	if a.MaxSubIter > 0 {
		return a.MaxSubIter
	}
	return defaultMaxSubIter
}

// Result is what Run returns.
type Result struct {
	Reply      string
	Status     string // complete | blocked | stalled | error | cancelled
	Iterations int
}

// Run executes the flat loop until complete_task, a stall, an error, or
// ctx cancellation. Flat by default; on thrash (tool errors, repeated same
// call, no progress) it injects ONE orientation turn (D36).
func (a *Agent) Run(ctx context.Context, msgs []provider.Message) *Result {
	system := a.System
	if a.Mem != nil {
		if b, err := a.Mem.Bootstrap(); err == nil {
			system = b.System() + system // constitution first, then task-specific
		}
	}
	if a.Skills != nil {
		if core, ok := a.Skills.PonytailCore(); ok {
			system = core.Body + "\n\n" + system // D30: always-loaded discipline
		}
		system = a.Skills.Descriptions() + "\n" + system
	}
	if system != "" && (len(msgs) == 0 || msgs[0].Role != "system") {
		msgs = append([]provider.Message{{Role: "system", Content: system}}, msgs...)
	}
	if a.Skills != nil {
		// D56 (cache-correct): matched skill bodies are APPENDED after the
		// real system message, right before the current user turn — never
		// injected into the system prompt, which must stay byte-stable so
		// the provider prefix cache keeps hitting.
		if matched := a.Skills.Match(lastUserContent(msgs)); len(matched) > 0 {
			var sb strings.Builder
			for _, s := range matched {
				sb.WriteString("\n[skill loaded: " + s.Name + "]\n" + s.Body)
			}
			if len(msgs) > 1 {
				msgs = append(msgs[:len(msgs)-1], append(
					[]provider.Message{{Role: "user", Content: sb.String()}}, msgs[len(msgs)-1])...)
			}
		}
	}
	maxIter := a.MaxIter
	if maxIter == 0 {
		maxIter = defaultMaxIter
	}
	dedup := map[string]string{} // D32 layer 1: tool dedup (whole run, mino behavior)
	schemas := append(a.registry.Schemas(), completeSchema())
	oriented := false
	var errors, repeats, noProgress int
	var lastKey string

	for i := 1; i <= maxIter; i++ {
		if ctx.Err() != nil {
			return a.finish(&Result{Status: "cancelled", Reply: "cancelled", Iterations: i})
		}
		if s := a.takeSteer(); s != "" {
			msgs = append(msgs, provider.Message{Role: "user", Content: s}) // D58: delivered between iterations
			a.recordSteer(s)
		}
		// D58 (pi-style): a steer interrupts the in-flight LLM call so the
		// model sees it at its next turn without waiting for the batch to end.
		iterCtx, cancelIter := context.WithCancel(ctx)
		iterDone := make(chan struct{}) // closes the watcher when the iteration ends
		if a.steerCh != nil {
			go func() {
				select {
				case <-a.steerCh:
					cancelIter()
				case <-iterDone:
				case <-ctx.Done():
				}
			}()
		}
		resp, err := a.chat(iterCtx, provider.Request{Messages: msgs, Tools: schemas})
		cancelIter()
		close(iterDone)
		if err != nil {
			if ctx.Err() == nil {
				// steer-interrupt (or a stale signal): deliver the pending
				// message, then continue — the next call carries it
				if s := a.takeSteer(); s != "" {
					msgs = append(msgs, provider.Message{Role: "user", Content: s})
					a.recordSteer(s)
				}
				continue
			}
			return a.finish(&Result{Status: "cancelled", Reply: "cancelled", Iterations: i})
		}
		if a.Meter != nil {
			a.Meter.Record(resp.Usage) // D56: real token accounting per turn
		}
		if len(resp.Choices) == 0 ||
			(resp.Choices[0].Message.Content == "" && len(resp.Choices[0].Message.ToolCalls) == 0) {
			return a.finish(&Result{Status: "error", Reply: "(error: empty model response)", Iterations: i})
		}
		msg := resp.Choices[0].Message
		msgs = append(msgs, msg)

		// Completion protocol (D37): only complete_task can end the turn.
		if len(msg.ToolCalls) == 1 && msg.ToolCalls[0].Function.Name == completionToolName {
			status, reply := completionArgs(msg.ToolCalls[0].Function.Arguments)
			if (status == "complete" || status == "blocked") && reply != "" {
				return a.finish(&Result{Status: status, Reply: reply, Iterations: i})
			}
			msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: msg.ToolCalls[0].ID, Content: completionError})
			continue
		}

		if len(msg.ToolCalls) == 0 {
			noProgress++
			if !oriented && noProgress >= 3 {
				msgs = append(msgs, orientationMessage())
				oriented = true
				noProgress = 0
				continue
			}
			if oriented && noProgress >= 3 {
				return a.finish(&Result{Status: "stalled", Reply: "(stopped: repeated responses without completing the task)", Iterations: i})
			}
			// Conversational escape (D53): a chat turn — an answer or a
			// question back to the user — is a completed turn. If the model
			// still hasn't called complete_task after two nags, accept its
			// last prose as the answer instead of nagging forever (this is
			// what locked the dashboard input while the model "waited").
			if noProgress >= 2 {
				return a.finish(&Result{Status: "complete", Reply: msg.Content, Iterations: i})
			}
			msgs = append(msgs, provider.Message{Role: "user",
				Content: "Your previous response was pure text and did not end the turn. If you are answering the user conversationally or asking them a question, call complete_task ALONE with status complete (or blocked if you need their input) and your reply as the final message. Otherwise call the next tool."})
			continue
		}

		// Act: execute tool calls in model order; observe: feed results back.
		// Sequential execution is deliberate (D50): edits, shell commands, and
		// delegated work must not race without an explicit future decision.
		progress := false
		var executedKey string
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == completionToolName {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: completionError})
				continue
			}
			key := tc.Function.Name + "\x00" + canonicalArgs(tc.Function.Arguments)
			if tc.Function.Name == "shell" {
				key = "shell\x00" + normalizeCmd(commandArg(tc.Function.Arguments)) // D52: cosmetic variants dedup
			}
			if out, ok := dedup[key]; ok {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "[already executed] " + out})
				executedKey = key // repeat detection: cached calls are not progress
				if a.Observer != nil {
					a.Observer(Event{Kind: "tool", Text: tc.Function.Name, Status: "cached", Detail: eventDetail(out)})
				}
				continue
			}
			out, err := a.registry.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				errors++
				dedup[key] = "Error: " + err.Error() // D52: repeated failing commands dedup too
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "Error: " + err.Error()})
				if a.Observer != nil {
					a.Observer(Event{Kind: "tool", Text: tc.Function.Name, Status: "error", Detail: eventDetail(err.Error())})
				}
				continue
			}
			dedup[key] = out
			msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: out})
			progress = true
			executedKey = key
			if a.Observer != nil {
				a.Observer(Event{Kind: "tool", Text: tc.Function.Name, Status: "ok", Detail: eventDetail(out)})
			}
		}

		// Adaptive OODA (D36): flat by default; ONE orientation turn on
		// thrash; a successful tool call resets the episode.
		if progress {
			oriented, errors, repeats, noProgress = false, 0, 0, 0
			lastKey = ""
			continue
		}
		if executedKey != "" && executedKey == lastKey {
			repeats++
		} else {
			repeats = 0
			lastKey = executedKey
		}
		if oriented && errors >= 2 {
			return a.finish(&Result{Status: "stalled", Reply: "(stopped: repeated tool failures after orientation)", Iterations: i})
		}
		if (errors >= 2 || repeats >= 2) && !oriented {
			msgs = append(msgs, orientationMessage())
			oriented = true
			errors, repeats = 0, 0
		}
	}

	return a.finish(&Result{Status: "stalled", Reply: "(stopped: max iterations reached)", Iterations: maxIter})
}

func eventDetail(out string) string {
	runes := []rune(out)
	if len(runes) <= maxEventDetail {
		return out
	}
	const marker = "\n… evidence preview truncated …\n"
	half := (maxEventDetail - len([]rune(marker))) / 2
	return string(runes[:half]) + marker + string(runes[len(runes)-half:])
}

// lastUserContent returns the most recent user-role message content.
func lastUserContent(msgs []provider.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	return ""
}

// loadSkillTool lets the model fetch a skill body on demand (D27).
func (a *Agent) loadSkillTool() tools.Tool {
	return tools.Tool{
		Name:        "load_skill",
		Description: "Load a skill body by name. Skills are listed in the system prompt catalog.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			name, _ := args["name"].(string)
			if a.Skills == nil {
				return "", fmt.Errorf("error: no skills registry")
			}
			s, ok := a.Skills.ByName(name)
			if !ok {
				return "", fmt.Errorf("error: unknown skill %s", name)
			}
			return s.Body, nil
		},
	}
}

// Steer queues a mid-run user message (D58, pi-style): the loop delivers it
// at the next safe boundary — interrupting the in-flight LLM call if one is
// running. Latest-wins: a newer steer replaces a pending one.
func (a *Agent) Steer(msg string) {
	if msg == "" || a.steerCh == nil {
		return
	}
	a.steerMu.Lock()
	a.steer = msg
	a.steerMu.Unlock()
	select {
	case a.steerCh <- struct{}{}:
	default:
	}
}

// recordSteer remembers a delivered steer so the session history can
// include it (D58 — the dashboard and REPL drain it after the run).
func (a *Agent) recordSteer(s string) {
	a.steerMu.Lock()
	a.delivered = append(a.delivered, s)
	a.steerMu.Unlock()
}

// DrainedSteers returns and clears the steers delivered during the last run.
func (a *Agent) DrainedSteers() []string {
	a.steerMu.Lock()
	defer a.steerMu.Unlock()
	out := a.delivered
	a.delivered = nil
	return out
}

// takeSteer consumes and clears the pending steering message.
func (a *Agent) takeSteer() string {
	a.steerMu.Lock()
	defer a.steerMu.Unlock()
	s := a.steer
	a.steer = ""
	return s
}

// chat calls the LLM, streaming deltas through the observer when possible.
func (a *Agent) chat(ctx context.Context, req provider.Request) (*provider.Response, error) {
	if st, ok := a.LLM.(Streamer); ok && a.Observer != nil {
		return st.StreamChat(ctx, req,
			func(d string) { a.Observer(Event{Kind: "delta", Text: d}) },
			func(d string) { a.Observer(Event{Kind: "thinking", Text: d}) },
		)
	}
	resp, err := a.LLM.Chat(ctx, req)
	if err == nil && a.Observer != nil && len(resp.Choices) > 0 {
		a.Observer(Event{Kind: "delta", Text: resp.Choices[0].Message.Content})
		if rc := resp.Choices[0].Message.ReasoningContent; rc != "" {
			a.Observer(Event{Kind: "thinking", Text: rc})
		}
	}
	return resp, err
}

// finish emits the done event (reply carried for renderers that streamed
// nothing) and returns the result.
func (a *Agent) finish(res *Result) *Result {
	if a.Observer != nil {
		a.Observer(Event{Kind: "done", Status: res.Status, Text: res.Reply})
	}
	return res
}

func orientationMessage() provider.Message {
	return provider.Message{Role: "user", Content: orientationPrompt}
}

// normalizeCmd strips shell cosmetics so near-identical commands share a
// dedup key (D52): whitespace, quotes, and redirects/pipes are presentation,
// not intent — "go test ./..." vs "go test  ./... 2>&1 | tail" are the same
// command. This is what stops the obsessive re-run loop: the model's
// variations collapse, dedup returns "[already executed]", and thrash
// detection counts them as repeats.
func normalizeCmd(s string) string {
	s = strings.ReplaceAll(s, `"`, "")
	s = strings.ReplaceAll(s, "'", "")
	for _, sep := range []string{" 2>&1", " 2>", " >", " >>", " |"} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	return strings.Join(strings.Fields(s), " ")
}

// commandArg extracts the shell tool's "command" argument from JSON args.
func commandArg(argsJSON string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return argsJSON
	}
	cmd, _ := args["command"].(string)
	return cmd
}
