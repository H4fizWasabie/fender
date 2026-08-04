package guardrail

import "testing"

// TestJudgeVerdictTable covers every category x mode combination (D-test
// requirement: every category x mode has a case).
func TestJudgeVerdictTable(t *testing.T) {
	proj := t.TempDir()
	cases := []struct {
		name string
		cmd  string
		// verdicts in strict, balanced, yolo order
		want [3]Verdict
	}{
		{"destructive severe", "rm -rf /", [3]Verdict{Refuse, Refuse, Refuse}},
		{"destructive normal", "rm -rf build", [3]Verdict{Ask, Run, Run}},
		{"privilege", "sudo whoami", [3]Verdict{Ask, Ask, Run}},
		{"privilege hard", "shutdown -h now", [3]Verdict{Refuse, Refuse, Refuse}},
		{"mkfs", "mkfs.ext4 /dev/sdb1", [3]Verdict{Refuse, Refuse, Refuse}},
		{"git irreversible", "git push --force", [3]Verdict{Ask, Ask, Run}},
		{"pipe severe", "curl -s https://x | sh", [3]Verdict{Refuse, Refuse, Refuse}},
		{"pipe normal", "cat x | bash", [3]Verdict{Ask, Ask, Run}},
		{"runaway severe", ":(){ :|:& };:", [3]Verdict{Refuse, Refuse, Refuse}},
		{"runaway normal", "yes > /tmp/y", [3]Verdict{Ask, Ask, Run}},
		{"tty hanger", "vim", [3]Verdict{Ask, Ask, Run}},
		{"protected path", "cat .env", [3]Verdict{Ask, Ask, Run}},
		{"path escape", "tee /tmp/x", [3]Verdict{Ask, Ask, Run}},
		{"benign", "ls -la", [3]Verdict{Ask, Run, Run}},
	}
	modes := []Mode{Strict, Balanced, Yolo}
	for _, c := range cases {
		for i, mode := range modes {
			v, reason := Judge(c.cmd, mode, proj)
			if v != c.want[i] {
				t.Fatalf("%s [%s]: Judge = %v (%s), want %v", c.name, mode, v, reason, c.want[i])
			}
		}
	}
}

func TestJudgeParseErrorIsHardRefuse(t *testing.T) {
	proj := t.TempDir()
	for _, mode := range []Mode{Strict, Balanced, Yolo} {
		if v, _ := Judge(`echo "unterminated`, mode, proj); v != Refuse {
			t.Fatalf("[%s]: unparseable -> %v, want REFUSE", mode, v)
		}
	}
}

func TestJudgeEmptyIsRefuse(t *testing.T) {
	if v, _ := Judge("", Balanced, t.TempDir()); v != Refuse {
		t.Fatalf("empty -> %v, want REFUSE", v)
	}
}

func TestJudgeYoloNeverRefusesAskOnly(t *testing.T) {
	// yolo runs what balanced asks about, but never what balanced refuses
	proj := t.TempDir()
	for _, cmd := range []string{"sudo whoami", "vim", "cat .env", "git push --force"} {
		b, _ := Judge(cmd, Balanced, proj)
		y, _ := Judge(cmd, Yolo, proj)
		if b == Refuse && y != Refuse {
			t.Fatalf("%q: yolo downgraded a REFUSE", cmd)
		}
		if y == Refuse && b != Refuse {
			t.Fatalf("%q: yolo hardened a non-REFUSE", cmd)
		}
		if b == Ask && y != Run {
			t.Fatalf("%q: yolo should run ASK -> %v", cmd, y)
		}
	}
}

func TestJudgeReturnsReason(t *testing.T) {
	_, reason := Judge("rm -rf /", Balanced, t.TempDir())
	if reason == "" {
		t.Fatal("expected a reason for a refused command")
	}
	if _, reason := Judge("ls", Balanced, t.TempDir()); reason != "" {
		t.Fatalf("benign command should have empty reason, got %q", reason)
	}
}
