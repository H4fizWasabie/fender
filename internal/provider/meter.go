package provider

// Meter tracks real token usage across a session (D56, pi-style): every
// response's usage is accumulated, and the context meter derives
// CH (cache-hit rate) and window usage from actual provider numbers —
// never char estimation.
type Meter struct {
	InputTokens     int
	OutputTokens    int
	CacheReadTokens int
	// Last is the most recent request's accounting — the current context
	// usage is "what the model just saw".
	Last struct {
		PromptTokens  int
		CacheRead     int
	}
	Window int // provider context_window (0 = unknown)
	Reserve int // tokens reserved for the response (default 16384)
}

func (m *Meter) Record(u Usage) {
	m.InputTokens += u.PromptTokens
	m.OutputTokens += u.CompletionTokens
	m.CacheReadTokens += u.Cached()
	m.Last.PromptTokens = u.PromptTokens
	m.Last.CacheRead = u.Cached()
}

// CacheHitRate is the latest request's cache-hit percentage (pi's CH).
func (m *Meter) CacheHitRate() float64 {
	if m.Last.PromptTokens <= 0 {
		return 0
	}
	return float64(m.Last.CacheRead) / float64(m.Last.PromptTokens) * 100
}

// UsagePercent is the current context usage vs the window (pi's 61.9%/1.0M).
func (m *Meter) UsagePercent() float64 {
	if m.Window <= 0 || m.Last.PromptTokens <= 0 {
		return 0
	}
	return float64(m.Last.PromptTokens) / float64(m.Window) * 100
}

// NearLimit reports whether the context is within Reserve tokens of the
// window — the warning threshold (pi: contextTokens > window - reserve).
func (m *Meter) NearLimit() bool {
	return m.Window > 0 && m.Last.PromptTokens > 0 &&
		m.Last.PromptTokens > m.Window-m.Reserve
}

// Summary is the JSON-friendly snapshot for the UI (D56 meter display).
func (m *Meter) Summary() map[string]any {
	return map[string]any{
		"input_tokens":    m.InputTokens,
		"output_tokens":   m.OutputTokens,
		"cache_read":      m.CacheReadTokens,
		"cache_hit_rate":  round1(m.CacheHitRate()),
		"usage_percent":   round1(m.UsagePercent()),
		"window":          m.Window,
		"near_limit":      m.NearLimit(),
	}
}

func round1(f float64) float64 {
	return float64(int(f*10)) / 10
}
