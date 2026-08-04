package tools

import (
	"context"
	"strings"
	"testing"
)

func TestRegistryExecute(t *testing.T) {
	reg := &Registry{tools: map[string]Tool{}}
	reg.Add(Tool{
		Name:        "echo",
		Description: "echo back",
		Parameters:  map[string]any{"type": "object"},
		Call: func(ctx context.Context, args map[string]any) (string, error) {
			return "hi", nil
		},
	})
	out, err := reg.Execute(context.Background(), "echo", "{}")
	if err != nil || out != "hi" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if _, err := reg.Execute(context.Background(), "nope", "{}"); err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if _, err := reg.Execute(context.Background(), "echo", "{bad json"); err == nil {
		t.Fatal("expected error for malformed args")
	}
}

func TestRegistrySchemas(t *testing.T) {
	reg := &Registry{tools: map[string]Tool{}}
	reg.Add(Tool{
		Name:        "echo",
		Description: "echo back",
		Parameters:  map[string]any{"type": "object"},
		Call:        func(ctx context.Context, args map[string]any) (string, error) { return "", nil },
	})
	schemas := reg.Schemas()
	if len(schemas) != 1 {
		t.Fatalf("schemas = %d", len(schemas))
	}
	s := schemas[0]
	if s.Type != "function" || s.Function.Name != "echo" || s.Function.Description == "" || s.Function.Parameters == nil {
		t.Fatalf("schema = %+v", s)
	}
}

func TestRegistryWithout(t *testing.T) {
	reg := &Registry{tools: map[string]Tool{}}
	for _, n := range []string{"a", "b", "c"} {
		reg.Add(Tool{Name: n, Call: func(ctx context.Context, args map[string]any) (string, error) { return "", nil }})
	}
	sub := reg.Without("b")
	if got := strings.Join(sub.Names(), ","); got != "a,c" {
		t.Fatalf("sub = %q", got)
	}
	if got := strings.Join(reg.Names(), ","); got != "a,b,c" {
		t.Fatalf("original mutated: %q", got)
	}
}
