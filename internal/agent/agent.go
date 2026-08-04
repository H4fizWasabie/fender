// Package agent implements D13: ONE loop. Agent{LLM, Tools} runs the
// canonical loop (prompt -> LLM -> tool call -> execute -> result -> repeat).
// Subagents are the same type in a goroutine (delegate tool, Task 7).
package agent

import (
	"context"
	"fmt"

	ctxpkg "github.com/H4fizWasabie/fender/internal/context"
	"github.com/H4fizWasabie/fender/internal/memory"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

// LLM is what the loop needs from the provider layer. *provider.Client
// satisfies it (it fills the model when Request.Model is empty); tests use
// a scripted fake — the second implementation that justifies the interface.
type LLM interface {
	Chat(ctx context.Context, req provider.Request) (*provider.Response, error)
}

const (
	defaultMaxIter    = 30
	defaultMaxSubIter = 6
	orientationPrompt = `(orientation turn — harness-enforced) Stop and orient before acting again. Reply with exactly four points: 1) what you know, 2) what is uncertain, 3) your hypothesis for the failures so far, 4) the single next distinct action. Do not repeat a failed or already-executed call.`
)

// Agent is one loop: model + tools + discipline. The same type runs the
// parent and every subagent (D13).
type Agent struct {
	LLM        LLM
	Resolver   Resolver // subagent provider selection (D7); nil -> inherit parent LLM
	System     string
	MaxIter    int             // 0 -> defaultMaxIter
	MaxSubIter int             // 0 -> defaultMaxSubIter
	Ctx        *ctxpkg.Manager // D31 artifact layer; nil = ticket-03 behavior
	Mem        *memory.Memory // D39 ICM memory workspace; nil = ticket-04 behavior
	registry   *tools.Registry
}

// NewAgent wires llm to reg and registers the delegate tool (D13).
func NewAgent(llm LLM, reg *tools.Registry) *Agent {
	a := &Agent{LLM: llm, registry: reg}
	a.registry.Add(a.delegateTool())
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
	if a.Mem != nil {
		if b, err := a.Mem.Bootstrap(); err == nil {
			a.System = b.System() + a.System // constitution first, then task-specific
		}
	}
	if a.Ctx != nil {
		a.Ctx.Cleanup(ctxpkg.SweepAge)
		msgs = a.Ctx.For(a.System, msgs)
	}
	if a.System != "" && (len(msgs) == 0 || msgs[0].Role != "system") {
		msgs = append([]provider.Message{{Role: "system", Content: a.System}}, msgs...)
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
			return &Result{Status: "cancelled", Reply: "cancelled", Iterations: i}
		}
		resp, err := a.LLM.Chat(ctx, provider.Request{Messages: msgs, Tools: schemas})
		if err != nil {
			return &Result{Status: "error", Reply: fmt.Sprintf("(error: %v)", err), Iterations: i}
		}
		if len(resp.Choices) == 0 ||
			(resp.Choices[0].Message.Content == "" && len(resp.Choices[0].Message.ToolCalls) == 0) {
			return &Result{Status: "error", Reply: "(error: empty model response)", Iterations: i}
		}
		msg := resp.Choices[0].Message
		msgs = append(msgs, msg)

		// Completion protocol (D37): only complete_task can end the turn.
		if len(msg.ToolCalls) == 1 && msg.ToolCalls[0].Function.Name == completionToolName {
			status, reply := completionArgs(msg.ToolCalls[0].Function.Arguments)
			if (status == "complete" || status == "blocked") && reply != "" {
				return &Result{Status: status, Reply: reply, Iterations: i}
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
				return &Result{Status: "stalled", Reply: "(stopped: repeated responses without completing the task)", Iterations: i}
			}
			msgs = append(msgs, provider.Message{Role: "user",
				Content: "Your previous response contained no tool call and did not complete the task. Call the next tool, or call complete_task alone with status complete|blocked and the final reply."})
			continue
		}

		// Act: execute each tool call; observe: feed results back.
		progress := false
		var executedKey string
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == completionToolName {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: completionError})
				continue
			}
			key := tc.Function.Name + "\x00" + canonicalArgs(tc.Function.Arguments)
			if out, ok := dedup[key]; ok {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "[already executed] " + out})
				executedKey = key // repeat detection: cached calls are not progress
				continue
			}
			out, err := a.registry.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				errors++
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "Error: " + err.Error()})
				continue
			}
			if a.Ctx != nil {
				out = a.Ctx.CompactOutput(tc.Function.Name, out)
			}
			dedup[key] = out
			msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: out})
			progress = true
			executedKey = key
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
			return &Result{Status: "stalled", Reply: "(stopped: repeated tool failures after orientation)", Iterations: i}
		}
		if (errors >= 2 || repeats >= 2) && !oriented {
			msgs = append(msgs, orientationMessage())
			oriented = true
			errors, repeats = 0, 0
		}
	}

	return &Result{Status: "stalled", Reply: "(stopped: max iterations reached)", Iterations: maxIter}
}

func orientationMessage() provider.Message {
	return provider.Message{Role: "user", Content: orientationPrompt}
}
