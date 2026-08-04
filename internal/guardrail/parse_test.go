package guardrail

import (
	"strings"
	"testing"

	"mvdan.cc/sh/v3/syntax"
)

func TestParseCmd(t *testing.T) {
	if _, err := parseCmd("echo hi"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseCmd("echo \"unterminated"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestWordLiteral(t *testing.T) {
	word := func(src string) *syntax.Word {
		f, err := parseCmd(src)
		if err != nil {
			t.Fatal(err)
		}
		// first CallExpr's first arg word
		var w *syntax.Word
		syntax.Walk(f, func(n syntax.Node) bool {
			if c, ok := n.(*syntax.CallExpr); ok && len(c.Args) > 0 && w == nil {
				w = c.Args[0]
			}
			return true
		})
		return w
	}
	cases := []struct {
		src   string
		want  string
		known bool
	}{
		{`rm`, "rm", true},
		{`"rm"`, "rm", true},
		{`'a b'`, "a b", true},
		{`$HOME`, "", false},
		{`~/x`, "~/x", true},
		{`a$(b)c`, "", false},
		{`*.go`, "*.go", true}, // plain globs are Lit parts in mvdan's AST
	}
	for _, c := range cases {
		w := word(c.src)
		if w == nil {
			t.Fatalf("%s: no word found", c.src)
		}
		got, known := wordLiteral(w)
		if got != c.want || known != c.known {
			t.Fatalf("wordLiteral(%s) = %q, %v; want %q, %v", c.src, got, known, c.want, c.known)
		}
	}
}

func TestCallName(t *testing.T) {
	f, err := parseCmd("/usr/bin/rm -rf /")
	if err != nil {
		t.Fatal(err)
	}
	var call *syntax.CallExpr
	syntax.Walk(f, func(n syntax.Node) bool {
		if c, ok := n.(*syntax.CallExpr); ok {
			call = c
		}
		return true
	})
	name, known := callName(call)
	if !known || name != "/usr/bin/rm" {
		t.Fatalf("callName = %q, %v", name, known)
	}
}

func TestLiteralArgs(t *testing.T) {
	f, _ := parseCmd(`git push --force "$BRANCH"`)
	var call *syntax.CallExpr
	syntax.Walk(f, func(n syntax.Node) bool {
		if c, ok := n.(*syntax.CallExpr); ok {
			call = c
		}
		return true
	})
	args := literalArgs(call)
	if len(args) != 2 || args[0] != "push" || args[1] != "--force" {
		t.Fatalf("literalArgs = %v", args)
	}
}

func TestNonFlagArgs(t *testing.T) {
	got := nonFlagArgs([]string{"-rf", "--", "-x", "a", "-b"})
	want := []string{"-x", "a", "-b"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("nonFlagArgs = %v", got)
	}
}
