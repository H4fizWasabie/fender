// Package agent implements D13: ONE loop. Agent{LLM, Tools} runs the
// canonical loop (prompt -> LLM -> tool call -> execute -> result -> repeat).
// Subagents are the same type in a goroutine (delegate tool, Task 7).
package agent

import (
	"context"
	"fmt"

	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

// LLM is what the loop needs from the provider layer. *provider.Client
// satisfies it (it fills the model when Request.Model is empty); tests use
// a scripted fake — the second implementation that justifies the interface.
type LLM interface {
	Chat(ctx context.Context, req provider.Request) (*provider.Response, error)
}

const defaultMaxIter = 30

// Agent is one loop: model + tools + discipline. The same type runs the
// parent and every subagent (D13).
type Agent struct {
	LLM      LLM
	System   string
	MaxIter  int // 0 -> defaultMaxIter
	registry *tools.Registry
}

// NewAgent wires llm to reg. Task 7 registers the delegate tool here.
func NewAgent(llm LLM, reg *tools.Registry) *Agent {
	return &Agent{LLM: llm, registry: reg}
}

// Result is what Run returns.
type Result struct {
	Reply      string
	Status     string // complete | blocked | stalled | error | cancelled
	Iterations int
}

// Run executes the flat loop until complete_task, a stall, an error, or
// ctx cancellation. msgs is the conversation so far; a.System is prepended
// as the system message when set.
func (a *Agent) Run(ctx context.Context, msgs []provider.Message) *Result {
	if a.System != "" && (len(msgs) == 0 || msgs[0].Role != "system") {
		msgs = append([]provider.Message{{Role: "system", Content: a.System}}, msgs...)
	}
	maxIter := a.MaxIter
	if maxIter == 0 {
		maxIter = defaultMaxIter
	}
	dedup := map[string]string{} // D32 layer 1: tool dedup (whole run, mino behavior)
	schemas := append(a.registry.Schemas(), completeSchema())
	noProgress := 0

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
			if noProgress >= 3 {
				return &Result{Status: "stalled", Reply: "(stopped: repeated responses without completing the task)", Iterations: i}
			}
			msgs = append(msgs, provider.Message{Role: "user",
				Content: "Your previous response contained no tool call and did not complete the task. Call the next tool, or call complete_task alone with status complete|blocked and the final reply."})
			continue
		}

		// Act: execute each tool call; observe: feed results back.
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == completionToolName {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: completionError})
				continue
			}
			key := tc.Function.Name + "\x00" + canonicalArgs(tc.Function.Arguments)
			if out, ok := dedup[key]; ok {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "[already executed] " + out})
				continue
			}
			out, err := a.registry.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: "Error: " + err.Error()})
				continue
			}
			dedup[key] = out
			msgs = append(msgs, provider.Message{Role: "tool", ToolCallID: tc.ID, Content: out})
		}
	}

	return &Result{Status: "stalled", Reply: "(stopped: max iterations reached)", Iterations: maxIter}
}
