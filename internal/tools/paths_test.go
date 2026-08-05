
package tools

import (
	"strings"
	"testing"
)

func TestInProjectAllowsDotFender(t *testing.T) {
	// D57: .fender/ is part of the project — memory files must be readable
	// (a "." projectDir previously mis-rejected them via the "./" prefix).
	abs, err := inProject(".", ".fender/memory/PROJECT.md")
	if err != nil {
		t.Fatalf(".fender path rejected: %v", err)
	}
	if !strings.HasSuffix(abs, ".fender/memory/PROJECT.md") {
		t.Fatalf("resolved = %q", abs)
	}
}

func TestInProjectStillRejectsOutside(t *testing.T) {
	if _, err := inProject(".", "/etc/passwd"); err == nil {
		t.Fatal("absolute outside path must be rejected")
	}
	if _, err := inProject(".", "../sibling"); err == nil {
		t.Fatal("parent escape must be rejected")
	}
}
