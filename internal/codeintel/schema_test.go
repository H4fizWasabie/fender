package codeintel

import "testing"

func TestSpecFor(t *testing.T) {
	cases := []struct {
		path string
		want string
		ok   bool
	}{
		{"a.go", "go", true},
		{"b.py", "python", true},
		{"c.ts", "typescript", true},
		{"d.tsx", "typescript", true},
		{"e.js", "javascript", true},
		{"f.jsx", "javascript", true},
		{"g.rs", "", false},
		{"h.txt", "", false},
	}
	for _, c := range cases {
		spec, ok := specFor(c.path)
		if ok != c.ok || (ok && len(spec.name) == 0) {
			t.Fatalf("%s: ok=%v spec=%+v", c.path, ok, spec)
		}
	}
}
