package skills

import (
	"strings"
	"testing"
)

func TestParseSingleLine(t *testing.T) {
	m, body, ok := parseFrontmatter("---\nname: tdd\ndescription: Test-driven development.\n---\n# Body\ncontent\n")
	if !ok || m.Name != "tdd" || m.Description != "Test-driven development." || !m.ModelInvokable {
		t.Fatalf("m=%+v body=%q ok=%v", m, body, ok)
	}
	if !strings.Contains(body, "# Body") {
		t.Fatalf("body = %q", body)
	}
}

func TestParseQuoted(t *testing.T) {
	m, _, ok := parseFrontmatter("---\nname: implement\ndescription: \"Implement a piece of work based on a spec.\"\n---\n")
	if !ok || m.Description != "Implement a piece of work based on a spec." {
		t.Fatalf("m=%+v ok=%v", m, ok)
	}
}

func TestParseFolded(t *testing.T) {
	m, _, ok := parseFrontmatter("---\nname: ponytail\ndescription: >\n  Forces the laziest solution that actually works, simplest, shortest,\n  most minimal.\n---\n")
	if !ok || m.Name != "ponytail" {
		t.Fatalf("m=%+v ok=%v", m, ok)
	}
	if !strings.Contains(m.Description, "laziest solution") || !strings.Contains(m.Description, "most minimal") {
		t.Fatalf("description = %q", m.Description)
	}
}

func TestParseUserInvoked(t *testing.T) {
	m, _, ok := parseFrontmatter("---\nname: ask-matt\ndescription: Router skill.\ndisable-model-invocation: true\n---\n")
	if !ok || m.ModelInvokable {
		t.Fatalf("m=%+v ok=%v", m, ok)
	}
}

func TestParseBroken(t *testing.T) {
	if _, _, ok := parseFrontmatter("no frontmatter here"); ok {
		t.Fatal("expected failure")
	}
	if _, _, ok := parseFrontmatter("---\nname: x\n---\n"); ok {
		t.Fatal("missing description must fail")
	}
}

func TestAllBundledParse(t *testing.T) {
	reg, err := Bundled()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reg.all); got != 23 {
		t.Fatalf("bundled skills = %d, want 23", got)
	}
	for name, s := range reg.all {
		if s.Name == "" || s.Description == "" || s.Body == "" {
			t.Fatalf("skill %s incomplete: %+v", name, s)
		}
	}
}
