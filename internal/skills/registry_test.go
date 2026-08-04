package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, dir, name, desc string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0700); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + desc + "\n---\nbody of " + name
	if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingDirIsEmpty(t *testing.T) {
	r, err := Load(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.all) != 0 {
		t.Fatalf("all = %d", len(r.all))
	}
}

func TestLoadSkipsBroken(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "good", "Good skill.")
	bad := filepath.Join(dir, "bad")
	os.MkdirAll(bad, 0700)
	os.WriteFile(filepath.Join(bad, "SKILL.md"), []byte("no frontmatter"), 0600)
	r, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.ByName("good"); !ok {
		t.Fatal("good skill missing")
	}
	if _, ok := r.ByName("bad"); ok {
		t.Fatal("broken skill must be skipped")
	}
}

func TestMergeShadowing(t *testing.T) {
	bundled := &Registry{all: map[string]Skill{
		"x": {Name: "x", Description: "bundled x", Body: "b", Source: "bundled"},
	}}
	user := &Registry{all: map[string]Skill{
		"x": {Name: "x", Description: "user x", Body: "u", Source: "user"},
		"y": {Name: "y", Description: "user y", Body: "u", Source: "user"},
	}}
	project := &Registry{all: map[string]Skill{
		"x": {Name: "x", Description: "project x", Body: "p", Source: "project"},
	}}
	merged := bundled.Merge(project, user)
	if got, _ := merged.ByName("x"); got.Source != "project" {
		t.Fatalf("x = %+v", got)
	}
	if got, _ := merged.ByName("y"); got.Source != "user" {
		t.Fatalf("y = %+v", got)
	}
	if got, _ := merged.ByName("z"); got.Source != "bundled" {
		t.Fatalf("z = %+v", got)
	}
}

func TestDescriptionsCapped(t *testing.T) {
	reg := &Registry{all: map[string]Skill{
		"a": {Name: "a", Description: strings.Repeat("d", 3000)},
		"b": {Name: "b", Description: strings.Repeat("e", 3000)},
	}}
	got := reg.Descriptions()
	if len(got) > DescriptionListCap {
		t.Fatalf("descriptions %d > cap %d", len(got), DescriptionListCap)
	}
}

func TestPonytailCore(t *testing.T) {
	reg, err := Bundled()
	if err != nil {
		t.Fatal(err)
	}
	s, ok := reg.PonytailCore()
	if !ok || s.Name != "ponytail" {
		t.Fatalf("ponytail core = %+v ok=%v", s, ok)
	}
	if !strings.Contains(s.Body, "ladder") && !strings.Contains(s.Body, "laziest") {
		t.Fatalf("ponytail body suspicious: %q", s.Body[:100])
	}
}
