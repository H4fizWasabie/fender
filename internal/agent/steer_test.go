package agent

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/H4fizWasabie/fender/internal/guardrail"
	"github.com/H4fizWasabie/fender/internal/provider"
	"github.com/H4fizWasabie/fender/internal/tools"
)

// blockingLLM blocks on the first Chat until its ctx is cancelled (simulating
// a mid-generation steer interrupt), then succeeds on the second call.
type blockingLLM struct {
	first   bool
	second  bool
	release chan struct{}
}

func (b *blockingLLM) Chat(ctx context.Context, req provider.Request) (*provider.Response, error) {
	if !b.first {
		b.first = true
		<-ctx.Done() // the steer's interrupt cancels this call
		return nil, ctx.Err()
	}
	b.second = true
	return completeReply("complete", "redirected done"), nil
}

// D58: a steer interrupts the in-flight LLM call and is delivered as a user
// message at the next iteration.
func TestSteerInterruptsAndInjects(t *testing.T) {
	f := &blockingLLM{release: make(chan struct{})}
	proj := t.TempDir()
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	done := make(chan *Result, 1)
	go func() {
		done <- a.Run(context.Background(), []provider.Message{{Role: "user", Content: "do the task"}})
	}()
	time.Sleep(100 * time.Millisecond) // let the first call start blocking
	a.Steer("actually check the other folder first")
	select {
	case res := <-done:
		if res.Status != "complete" {
			t.Fatalf("status = %q", res.Status)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not finish after steer")
	}
	if !f.second {
		t.Fatal("second LLM call never happened")
	}
}

// latest-wins: two steers collapse into the newest one.
func TestSteerLatestWins(t *testing.T) {
	a := &Agent{steerCh: make(chan struct{}, 1)}
	a.Steer("first")
	a.Steer("second")
	if got := a.takeSteer(); got != "second" {
		t.Fatalf("steer = %q", got)
	}
	if got := a.takeSteer(); got != "" {
		t.Fatalf("steer not cleared: %q", got)
	}
}

// an empty steer is ignored.
func TestSteerEmptyIgnored(t *testing.T) {
	a := &Agent{steerCh: make(chan struct{}, 1)}
	a.Steer("")
	if got := a.takeSteer(); got != "" {
		t.Fatalf("steer = %q", got)
	}
}

// gatedLLM: first call returns a tool call; the second call blocks until
// released — giving the test a deterministic window to steer mid-run.
type gatedLLM struct {
	mu    sync.Mutex
	gate  chan struct{}
	calls int
	reqs  []provider.Request
}

func (g *gatedLLM) Chat(ctx context.Context, req provider.Request) (*provider.Response, error) {
	g.mu.Lock()
	g.reqs = append(g.reqs, req)
	g.calls++
	n := g.calls
	g.mu.Unlock()
	switch n {
	case 1:
		return toolReply("c1", "shell", `{"command":"echo step1"}`), nil
	case 2:
		<-g.gate // test releases; the steer interrupts this call
		<-ctx.Done()
		return nil, ctx.Err()
	default:
		return completeReply("complete", "done"), nil
	}
}

func (g *gatedLLM) all() []provider.Request {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]provider.Request(nil), g.reqs...)
}

// a steer arriving mid-run (while the LLM is generating) interrupts the
// call and is delivered at the next iteration (D58).
func TestSteerMidGeneration(t *testing.T) {
	g := &gatedLLM{gate: make(chan struct{})}
	proj := t.TempDir()
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(g, reg)
	done := make(chan *Result, 1)
	go func() {
		done <- a.Run(context.Background(), []provider.Message{{Role: "user", Content: "task"}})
	}()
	// wait until the second call (the blocked generation) is in flight
	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		if len(g.all()) >= 2 {
			break
		}
	}
	a.Steer("check the other folder first")
	close(g.gate) // release the blocked call — the steer interrupts it
	res := <-done
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	reqs := g.all()
	joined := ""
	for _, m := range reqs[len(reqs)-1].Messages {
		joined += m.Content
	}
	if !strings.Contains(joined, "check the other folder first") {
		t.Fatalf("steer not delivered: %q", joined)
	}
}

// a steer sent after the run completed stays pending for the next run.
func TestSteerStaysPendingAfterCompletion(t *testing.T) {
	f := &fakeLLM{steps: []*provider.Response{completeReply("complete", "done")}}
	proj := t.TempDir()
	reg := tools.New(proj, tools.ShellConfig{Mode: guardrail.Balanced, ProjectDir: proj}, nil)
	a := NewAgent(f, reg)
	res := a.Run(context.Background(), []provider.Message{{Role: "user", Content: "task"}})
	if res.Status != "complete" {
		t.Fatalf("status = %q", res.Status)
	}
	a.Steer("next task please")
	if got := a.takeSteer(); got != "next task please" {
		t.Fatalf("pending steer = %q", got)
	}
}
