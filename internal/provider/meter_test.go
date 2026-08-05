package provider

import "testing"

func TestMeterRecordsUsage(t *testing.T) {
	m := &Meter{Window: 1_000_000, Reserve: 16384}
	m.Record(Usage{PromptTokens: 100_000, CompletionTokens: 1000, CacheReadTokens: 95_000})
	if m.InputTokens != 100_000 || m.OutputTokens != 1000 || m.CacheReadTokens != 95_000 {
		t.Fatalf("totals = %+v", m)
	}
	if m.CacheHitRate() != 95.0 {
		t.Fatalf("CH = %v", m.CacheHitRate())
	}
	if m.UsagePercent() != 10.0 {
		t.Fatalf("usage = %v", m.UsagePercent())
	}
	if m.NearLimit() {
		t.Fatal("not near limit")
	}
}

func TestMeterOpenAICachedTokens(t *testing.T) {
	var u Usage
	u.PromptTokens = 100
	u.Details.CachedTokens = 80
	m := &Meter{Window: 1000}
	m.Record(u)
	if m.CacheReadTokens != 80 {
		t.Fatalf("cache = %d", m.CacheReadTokens)
	}
	if m.CacheHitRate() != 80.0 {
		t.Fatalf("CH = %v", m.CacheHitRate())
	}
}

func TestMeterNearLimit(t *testing.T) {
	m := &Meter{Window: 100_000, Reserve: 16_384}
	m.Record(Usage{PromptTokens: 90_000})
	if !m.NearLimit() {
		t.Fatal("90K of 100K with 16K reserve must warn")
	}
	m.Record(Usage{PromptTokens: 50_000})
	if m.NearLimit() {
		t.Fatal("50K must not warn")
	}
}
