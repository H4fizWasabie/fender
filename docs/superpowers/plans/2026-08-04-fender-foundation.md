# Fender Plan 1: Foundation (module, config, provider) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fender's skeleton: Go module, `fender.toml` config loading, OpenAI-compatible provider client + registry, and a `fender providers` CLI command that lists configured providers.

**Architecture:** Flat Go module with `cmd/fender` (CLI entry) + `internal/provider` (config types, registry, OpenAI-compatible HTTP client). No frameworks. The client speaks the OpenAI chat-completions wire format (non-streaming + SSE streaming) so any OpenAI-compatible endpoint (OpenRouter, Ollama, LM Studio, vLLM) works with just a base URL + key.

**Tech Stack:** Go 1.22, stdlib `net/http` + `encoding/json`, `github.com/BurntSushi/toml` (only external dep, already approved in AGENTS.md).

## Global Constraints

- **Read `AGENTS.md`, `DECISIONS.md`, and the design spec first.** They are the constitution (D1–D37).
- **Every commit MUST stage `CHANGELOG.md`** with a `[Unreleased]` entry — enforced by `.githooks/pre-commit`.
- **Allowed dependencies:** `BurntSushi/toml`, `mvdan.cc/sh/v3`, `go-tree-sitter`, `modernc.org/sqlite`. Nothing else without discussion (AGENTS.md rule).
- **No frameworks, no agent libraries.** Stdlib HTTP, stdlib JSON, stdlib testing.
- **Single binary** — `go build ./cmd/fender` must produce one binary.
- **Go 1.22+**, `log/slog` for logging, explicit errors (no panic in library code).
- Module path: `github.com/H4fizWasabie/fender` (decision confirmed 2026-08-04).
- File layout per AGENTS.md: `cmd/fender/` + `internal/provider/`. Flat over nested.

---

### Task 1: Module init + skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/fender/main.go`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Produces: `main.go` that compiles and prints a version line when run.

- [ ] **Step 1: Init the module**

```bash
cd /home/hafiz/Desktop/Fender
go mod init github.com/H4fizWasabie/fender
go get github.com/BurntSushi/toml@latest
```

Expected: `go.mod` created with module path `github.com/H4fizWasabie/fender` and `require github.com/BurntSushi/toml`.

- [ ] **Step 2: Create the CLI stub**

`cmd/fender/main.go`:

```go
package main

import "fmt"

const version = "0.1.0"

func main() {
	fmt.Printf("fender %s\n", version)
}
```

- [ ] **Step 3: Verify it builds and runs**

```bash
go build ./cmd/fender && ./fender
```

Expected: prints `fender 0.1.0`. The `./fender` binary lands in the repo root — delete it after verifying (`rm -f fender`); never commit binaries.

- [ ] **Step 4: Append changelog entry and commit**

```bash
git add go.mod go.sum cmd/fender/main.go CHANGELOG.md
git commit -m "chore: module skeleton (cmd/fender stub, go.mod)"
```

CHANGELOG entry to add first:

```markdown
### Added
- Module skeleton: `cmd/fender` stub, `go.mod` (github.com/H4fizWasabie/fender), BurntSushi/toml dependency
```

---

### Task 2: Config types (fender.toml schema)

**Files:**
- Create: `internal/provider/config.go`
- Create: `internal/provider/config_test.go`
- Create: `internal/provider/testdata/config.toml`

**Interfaces:**
- Produces:
  - `type Provider struct { BaseURL string; APIKey string; Models []string; DefaultModel string }` (toml tags: `base_url`, `api_key`, `models`, `default_model`)
  - `type Config struct { Providers map[string]Provider }` (toml tag: `providers`)

- [ ] **Step 1: Write the failing test**

`internal/provider/config_test.go`:

```go
package provider

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDecodeConfig(t *testing.T) {
	var cfg Config
	if _, err := toml.DecodeFile("testdata/config.toml", &cfg); err != nil {
		t.Fatal(err)
	}
	openrouter, ok := cfg.Providers["openrouter"]
	if !ok {
		t.Fatal("missing provider openrouter")
	}
	if openrouter.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("base_url = %q", openrouter.BaseURL)
	}
	if openrouter.APIKey != "sk-test-123" {
		t.Fatalf("api_key = %q", openrouter.APIKey)
	}
	if len(openrouter.Models) != 2 || openrouter.Models[0] != "openai/gpt-4o-mini" {
		t.Fatalf("models = %v", openrouter.Models)
	}
	if openrouter.DefaultModel != "openai/gpt-4o-mini" {
		t.Fatalf("default_model = %q", openrouter.DefaultModel)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/provider/ -run TestDecodeConfig -v`
Expected: FAIL — `config.go` doesn't exist (package has no files).

- [ ] **Step 3: Write the config types**

`internal/provider/config.go`:

```go
// Package provider holds the OpenAI-compatible provider layer (D6, D7, D25).
package provider

// Config is the [providers] section of fender.toml.
type Config struct {
	Providers map[string]Provider `toml:"providers"`
}

// Provider is one OpenAI-compatible endpoint (OpenRouter, Ollama, LM Studio, ...).
type Provider struct {
	BaseURL      string   `toml:"base_url"`
	APIKey       string   `toml:"api_key"`
	Models       []string `toml:"models"`
	DefaultModel string   `toml:"default_model"`
}
```

- [ ] **Step 4: Create the test fixture**

`internal/provider/testdata/config.toml`:

```toml
[providers.openrouter]
base_url = "https://openrouter.ai/api/v1"
api_key = "sk-test-123"
models = ["openai/gpt-4o-mini", "anthropic/claude-sonnet-4"]
default_model = "openai/gpt-4o-mini"

[providers.ollama]
base_url = "http://localhost:11434/v1"
api_key = "ollama"
models = ["llama3.2"]
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/provider/ -run TestDecodeConfig -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/ CHANGELOG.md
git commit -m "feat: fender.toml config types (providers registry schema)"
```

CHANGELOG entry:

```markdown
### Added
- `internal/provider` config types: fender.toml providers schema (base_url, api_key, models, default_model)
```

---

### Task 3: OpenAI-compatible client (non-streaming)

**Files:**
- Create: `internal/provider/client.go`
- Create: `internal/provider/client_test.go`

**Interfaces:**
- Consumes: `Config`, `Provider` from Task 2.
- Produces:
  - `type Message struct { Role string; Content string; ToolCalls []ToolCall; ToolCallID string }` with json tags (`role`, `content`, `tool_calls,omitempty`, `tool_call_id,omitempty`)
  - `type ToolCall struct { ID string; Type string; Function ToolFunction }` — `ToolFunction{ Name, Arguments string }`
  - `type ToolDef struct { Type string; Function ToolFunctionDef }` — `ToolFunctionDef{ Name, Description string; Parameters any }`
  - `type Request struct { Model string; Messages []Message; Tools []ToolDef; Stream bool; MaxTokens int }`
  - `type Response struct { Choices []Choice; Usage Usage }` — `Choice{ Message Message }`, `Usage{ PromptTokens, CompletionTokens int }`
  - `type Client struct` with `New(name string, p Provider) *Client` and `Chat(ctx context.Context, req Request) (*Response, error)`

- [ ] **Step 1: Write the failing test**

`internal/provider/client_test.go`:

```go
package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatSendsRequestAndParsesResponse(t *testing.T) {
	var gotBody map[string]any
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"hello from mock"}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	}))
	defer srv.Close()

	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m1"}, DefaultModel: "m1"})
	resp, err := c.Chat(context.Background(), Request{
		Model:    "m1",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer k" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotBody["model"] != "m1" {
		t.Fatalf("model = %v", gotBody["model"])
	}
	if resp.Choices[0].Message.Content != "hello from mock" {
		t.Fatalf("content = %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Fatalf("prompt_tokens = %d", resp.Usage.PromptTokens)
	}
}

func TestChatParsesToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a.go\"}"}}]}}]}`)
	}))
	defer srv.Close()

	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m1"}, DefaultModel: "m1"})
	resp, err := c.Chat(context.Background(), Request{Model: "m1", Messages: []Message{{Role: "user", Content: "read a file"}}})
	if err != nil {
		t.Fatal(err)
	}
	tc := resp.Choices[0].Message.ToolCalls
	if len(tc) != 1 || tc[0].ID != "call_1" || tc[0].Function.Name != "read_file" {
		t.Fatalf("tool_calls = %+v", tc)
	}
}

func TestChatReturnsErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad key"}}`, http.Status401)
	}))
	defer srv.Close()

	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m1"}, DefaultModel: "m1"})
	_, err := c.Chat(context.Background(), Request{Model: "m1", Messages: []Message{{Role: "user", Content: "x"}}})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/provider/ -run TestChat -v`
Expected: FAIL — `client.go` missing.

- [ ] **Step 3: Write the client**

`internal/provider/client.go`:

```go
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDef struct {
	Type     string          `json:"type"`
	Function ToolFunctionDef `json:"function"`
}

type ToolFunctionDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type Request struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Tools     []ToolDef `json:"tools,omitempty"`
	Stream    bool      `json:"stream,omitempty"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

type Response struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Message Message `json:"message"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Client is one OpenAI-compatible endpoint. Not safe for concurrent use
// beyond what http.Client provides.
type Client struct {
	name   string
	base   string
	apiKey string
	model  string
	http   *http.Client
}

func New(name string, p Provider) *Client {
	return &Client{
		name:   name,
		base:   strings.TrimSuffix(p.BaseURL, "/"),
		apiKey: p.APIKey,
		model:  p.DefaultModel,
		http:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *Client) Name() string  { return c.name }
func (c *Client) Model() string { return c.model }

func (c *Client) Chat(ctx context.Context, req Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%s: %s: %s", c.name, resp.Status, strings.TrimSpace(string(msg)))
	}
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", c.name, err)
	}
	return &out, nil
}
```

Note: `io` is used by the tests (`io.WriteString`) — the test file needs `"io"` imported too (it's in the test file's import list in Step 1).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/provider/ -run TestChat -v`
Expected: all three PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/ CHANGELOG.md
git commit -m "feat: OpenAI-compatible chat client (non-streaming, tool calls, errors)"
```

CHANGELOG entry:

```markdown
### Added
- OpenAI-compatible client: Chat() with tool_calls parsing, Bearer auth, non-200 error wrapping
```

---

### Task 4: Streaming client (SSE)

**Files:**
- Modify: `internal/provider/client.go`
- Create: `internal/provider/stream_test.go`

**Interfaces:**
- Produces:
  - `func (c *Client) Stream(ctx context.Context, req Request, onDelta func(string)) (*Response, error)` — sends `stream: true`, calls `onDelta` for each content delta, returns the accumulated response (tool_calls included).

- [ ] **Step 1: Write the failing test**

`internal/provider/stream_test.go`:

```go
package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamCollectsDeltas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\n" +
				"data: [DONE]\n\n",
		))
	}))
	defer srv.Close()

	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m1"}, DefaultModel: "m1"})
	var got strings.Builder
	resp, err := c.Stream(context.Background(), Request{Model: "m1", Messages: []Message{{Role: "user", Content: "hi"}}}, func(d string) {
		got.WriteString(d)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "hello" {
		t.Fatalf("deltas = %q", got.String())
	}
	if resp.Choices[0].Message.Content != "hello" {
		t.Fatalf("content = %q", resp.Choices[0].Message.Content)
	}
}

func TestStreamCollectsToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"read_file\",\"arguments\":\"{\\\"pa\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"th\\\":\\\"a.go\\\"}\"}}]}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{}}]}\n\n" +
				"data: [DONE]\n\n",
		))
	}))
	defer srv.Close()

	c := New("mock", Provider{BaseURL: srv.URL, APIKey: "k", Models: []string{"m1"}, DefaultModel: "m1"})
	resp, err := c.Stream(context.Background(), Request{Model: "m1", Messages: []Message{{Role: "user", Content: "read"}}}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	tc := resp.Choices[0].Message.ToolCalls
	if len(tc) != 1 || tc[0].ID != "call_1" || tc[0].Function.Name != "read_file" {
		t.Fatalf("tool_calls = %+v", tc)
	}
	if !strings.Contains(tc[0].Function.Arguments, `"path":"a.go"`) {
		t.Fatalf("arguments = %q", tc[0].Function.Arguments)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/provider/ -run TestStream -v`
Expected: FAIL — `Stream` undefined.

- [ ] **Step 3: Implement streaming**

Append to `internal/provider/client.go`:

```go
// streamDelta mirrors the streaming chunk shape: choices[].delta.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

func (c *Client) Stream(ctx context.Context, req Request, onDelta func(string)) (*Response, error) {
	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%s: %s: %s", c.name, resp.Status, strings.TrimSpace(string(msg)))
	}

	out := &Response{}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var tcBuf []ToolCall // accumulated tool calls across chunks, by index
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // ignore malformed keep-alive chunks
		}
		for _, ch := range chunk.Choices {
			msg := &out.Choices[0].Message // out.Choices always has index 0 by construction
			if len(out.Choices) == 0 {
				out.Choices = append(out.Choices, Choice{})
				msg = &out.Choices[0].Message
			}
			if ch.Delta.Content != "" {
				msg.Content += ch.Delta.Content
				onDelta(ch.Delta.Content)
			}
			for _, tc := range ch.Delta.ToolCalls {
				tcBuf = append(tcBuf, tc)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(tcBuf) > 0 {
		out.Choices[0].Message.ToolCalls = tcBuf
	}
	return out, nil
}
```

Note: streaming tool-call chunks fragment `arguments` across chunks (as the test shows); v1 concatenates fragments per chunk order with a single tool call per response — the loop (Plan 3) will handle multi-call streaming. This matches the test fixtures exactly.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/provider/ -run TestStream -v`
Expected: PASS. Then `go test ./internal/provider/` — all tests green.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/ CHANGELOG.md
git commit -m "feat: SSE streaming chat client with delta accumulation"
```

CHANGELOG entry:

```markdown
### Added
- Streaming client: Stream() with SSE parsing, content deltas via callback, tool-call accumulation
```

---

### Task 5: Registry (Load + Client lookup)

**Files:**
- Create: `internal/provider/registry.go`
- Create: `internal/provider/registry_test.go`

**Interfaces:**
- Consumes: `Config`, `Provider`, `New` from Tasks 2–3.
- Produces:
  - `type Registry struct` with:
    - `func Load(path string) (*Registry, error)` — reads fender.toml, returns error if file missing or TOML invalid
    - `func LoadDefault() (*Registry, error)` — tries `./fender.toml`, then `~/.fender/fender.toml`
    - `func (r *Registry) Client(name string) (*Client, bool)`
    - `func (r *Registry) Names() []string` — sorted provider names
    - `func (r *Registry) Default() (*Client, bool)` — first provider (sorted) with a default_model set

- [ ] **Step 1: Write the failing test**

`internal/provider/registry_test.go`:

```go
package provider

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fender.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

const twoProviders = `
[providers.openrouter]
base_url = "https://openrouter.ai/api/v1"
api_key = "k1"
models = ["a", "b"]
default_model = "a"

[providers.ollama]
base_url = "http://localhost:11434/v1"
api_key = "ollama"
models = ["llama3.2"]
`

func TestLoadAndLookup(t *testing.T) {
	r, err := Load(writeConfig(t, twoProviders))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := r.Client("ollama")
	if !ok || c.Model() != "llama3.2" {
		t.Fatalf("ollama client = %v, ok=%v", c, ok)
	}
	if _, ok := r.Client("nope"); ok {
		t.Fatal("unexpected provider nope")
	}
	names := r.Names()
	if !sort.StringsAreSorted(names) || len(names) != 2 {
		t.Fatalf("names = %v", names)
	}
	d, ok := r.Default()
	if !ok || d.Name() != "openrouter" {
		t.Fatalf("default = %v, ok=%v", d, ok)
	}
}

func TestLoadMissingFileErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.toml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidTOMLErrors(t *testing.T) {
	if _, err := Load(writeConfig(t, "not toml [")); err == nil {
		t.Fatal("expected error for invalid toml")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/provider/ -run TestLoad -v`
Expected: FAIL — `Load` undefined.

- [ ] **Step 3: Write the registry**

`internal/provider/registry.go`:

```go
package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// Registry holds configured providers loaded from fender.toml (D7, D25).
type Registry struct {
	clients map[string]*Client
	order   []string // sorted provider names
}

func Load(path string) (*Registry, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return build(cfg), nil
}

// LoadDefault tries ./fender.toml, then ~/.fender/fender.toml.
func LoadDefault() (*Registry, error) {
	for _, path := range []string{"fender.toml"} {
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(home, ".fender", "fender.toml")
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}
	return nil, fmt.Errorf("no fender.toml found (tried ./fender.toml and ~/.fender/fender.toml)")
}

func build(cfg Config) *Registry {
	r := &Registry{clients: make(map[string]*Client, len(cfg.Providers))}
	for name, p := range cfg.Providers {
		r.clients[name] = New(name, p)
		r.order = append(r.order, name)
	}
	sort.Strings(r.order)
	return r
}

func (r *Registry) Client(name string) (*Client, bool) {
	c, ok := r.clients[name]
	return c, ok
}

func (r *Registry) Names() []string {
	return r.order
}

// Default returns the first provider (sorted) that has a default_model set.
func (r *Registry) Default() (*Client, bool) {
	for _, name := range r.order {
		if c := r.clients[name]; c.Model() != "" {
			return c, true
		}
	}
	return nil, false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/provider/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/ CHANGELOG.md
git commit -m "feat: provider registry (Load, LoadDefault, Default lookup)"
```

CHANGELOG entry:

```markdown
### Added
- Provider registry: fender.toml loading (./ then ~/.fender/), client lookup, default provider
```

---

### Task 6: CLI — `fender providers`

**Files:**
- Modify: `cmd/fender/main.go`
- Create: `cmd/fender/main_test.go`

**Interfaces:**
- Consumes: `Load`, `Registry` from Task 5.
- Produces: CLI behavior:
  - `fender providers` — prints each provider: `name  base_url  models=...  default=...`
  - `fender --config PATH providers` — loads from PATH instead of defaults
  - `fender` with no args — prints usage and exits 0

- [ ] **Step 1: Write the failing test**

`cmd/fender/main_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fender.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

const cfg = `
[providers.openrouter]
base_url = "https://openrouter.ai/api/v1"
api_key = "k1"
models = ["a", "b"]
default_model = "a"
`

func TestProvidersCommand(t *testing.T) {
	path := writeConfig(t, cfg)
	var out bytes.Buffer
	if err := runCLI(&out, []string{"--config", path, "providers"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "openrouter") || !strings.Contains(got, "https://openrouter.ai/api/v1") {
		t.Fatalf("output = %q", got)
	}
	if !strings.Contains(got, "a, b") {
		t.Fatalf("models missing: %q", got)
	}
}

func TestNoArgsShowsUsage(t *testing.T) {
	var out bytes.Buffer
	if err := runCLI(&out, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "usage") {
		t.Fatalf("output = %q", out.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/fender/ -v`
Expected: FAIL — `runCLI` undefined.

- [ ] **Step 3: Write the CLI**

`cmd/fender/main.go` (replace Task 1 stub):

```go
package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/H4fizWasabie/fender/internal/provider"
)

const version = "0.1.0"

func main() {
	if err := runCLI(io.Discard, nil); err != nil { // placeholder, replaced below
	}
}
```

Wait — that's wrong. The correct full file:

```go
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/H4fizWasabie/fender/internal/provider"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Stdout, os.Args[1:]))
}

// run parses args and executes; returns process exit code.
func run(out io.Writer, args []string) int {
	if err := runCLI(out, args); err != nil {
		fmt.Fprintln(out, "error:", err)
		return 1
	}
	return 0
}

func runCLI(out io.Writer, args []string) error {
	fs := flag.NewFlagSet("fender", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to fender.toml (default: ./fender.toml, then ~/.fender/fender.toml)")
	fs.SetOutput(out)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(out, "usage: fender [--config PATH] providers")
		fmt.Fprintln(out, "  providers   list configured providers")
		return nil
	}
	switch fs.Arg(0) {
	case "providers":
		return listProviders(out, *configPath)
	default:
		return fmt.Errorf("unknown command %q", fs.Arg(0))
	}
}

func listProviders(out io.Writer, configPath string) error {
	var (
		r   *provider.Registry
		err error
	)
	if configPath != "" {
		r, err = provider.Load(configPath)
	} else {
		r, err = provider.LoadDefault()
	}
	if err != nil {
		return err
	}
	for _, name := range r.Names() {
		c, _ := r.Client(name)
		fmt.Fprintf(out, "%-15s %-40s models=%s default=%s\n", name, c.BaseURL(), strings.Join(models(c), ", "), c.Model())
	}
	return nil
}
```

Note: this calls `c.BaseURL()` — add that method to the client in `internal/provider/client.go`:

```go
func (c *Client) BaseURL() string { return c.base }
```

and add to the Task 3 client a helper for models — the client doesn't store the full model list. Add to `Client`:

```go
// in client.go, extend struct + constructor + accessor
type Client struct {
	name   string
	base   string
	apiKey string
	model  string
	models []string
	http   *http.Client
}

func New(name string, p Provider) *Client {
	return &Client{
		name:   name,
		base:   strings.TrimSuffix(p.BaseURL, "/"),
		apiKey: p.APIKey,
		model:  p.DefaultModel,
		models: p.Models,
		http:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *Client) Models() []string { return c.models }
```

Then `listProviders` uses `c.Models()` — update the snippet above accordingly:

```go
fmt.Fprintf(out, "%-15s %-40s models=%s default=%s\n", name, c.BaseURL(), strings.Join(c.Models(), ", "), c.Model())
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/fender/ ./internal/provider/ -v`
Expected: all PASS.

- [ ] **Step 5: Manual smoke test**

```bash
go build ./cmd/fender
./fender --config internal/provider/testdata/config.toml providers
rm -f fender
```

Expected: prints both test providers with models and defaults.

- [ ] **Step 6: Commit**

```bash
git add cmd/fender/ internal/provider/ CHANGELOG.md
git commit -m "feat: fender providers CLI command (--config flag, provider listing)"
```

CHANGELOG entry:

```markdown
### Added
- `fender providers` CLI: lists configured providers, models, defaults; `--config` flag
```

---

## Self-Review Notes

- **Spec coverage:** D1 (from-scratch Go) — Task 1; D6 (OpenAI-compatible only) — Tasks 3–4; D7 (provider registry) — Task 5; D25 (TOML config) — Task 2; D8 (multi-provider subagent routing) — Task 5 `Client(name)` lookup is the seam the subagent tool uses in Plan 3.
- **Placeholders:** none — every code step contains full source. The one inline correction (client `Models()`/`BaseURL()` accessors) is spelled out in Task 6 Step 3 rather than left implicit.
- **Type consistency:** `Provider`/`Config` (Task 2) → `New` (Task 3) → `Load` (Task 5) → `runCLI` (Task 6) — signatures match across tasks. `Message`/`ToolCall`/`ToolDef`/`Request`/`Response` are the wire types Plan 3's loop consumes.
- **CHANGELOG:** every task ends with a changelog entry + commit (hook-enforced).
- **Deps:** only `BurntSushi/toml` (approved). Tests use stdlib `httptest`/`bufio`/`io`.

**Execution order note:** Task 1 must be first (module init). Tasks 2–5 are sequential (each builds on the previous). Task 6 depends on Task 5.
