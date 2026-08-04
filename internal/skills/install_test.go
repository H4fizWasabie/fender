package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInstallCopies(t *testing.T) {
	src := t.TempDir()
	writeSkill(t, src, "alpha", "Alpha skill.")
	writeSkill(t, src, "beta", "Beta skill.")
	// support file
	os.WriteFile(filepath.Join(src, "alpha", "notes.md"), []byte("support"), 0600)

	dest := filepath.Join(t.TempDir(), "skills")
	got, err := Install(src, dest)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("installed = %v", got)
	}
	if _, err := os.Stat(filepath.Join(dest, "alpha", "SKILL.md")); err != nil {
		t.Fatal("alpha SKILL.md missing")
	}
	if _, err := os.Stat(filepath.Join(dest, "alpha", "notes.md")); err != nil {
		t.Fatal("support file missing")
	}
}

func TestInstallIdempotent(t *testing.T) {
	src := t.TempDir()
	writeSkill(t, src, "alpha", "Alpha skill.")
	dest := filepath.Join(t.TempDir(), "skills")
	if _, err := Install(src, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, dest); err != nil {
		t.Fatal(err)
	}
}
