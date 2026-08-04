package guardrail

import "testing"

func TestParseMode(t *testing.T) {
	for _, ok := range []struct {
		in   string
		want Mode
	}{
		{"strict", Strict}, {"balanced", Balanced}, {"yolo", Yolo},
	} {
		m, err := ParseMode(ok.in)
		if err != nil || m != ok.want {
			t.Fatalf("ParseMode(%q) = %v, %v", ok.in, m, err)
		}
	}
	if _, err := ParseMode("paranoid"); err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestVerdictString(t *testing.T) {
	if Run.String() != "RUN" || Ask.String() != "ASK" || Refuse.String() != "REFUSE" {
		t.Fatal("verdict strings wrong")
	}
}
