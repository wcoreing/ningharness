package metadir

import (
	"path/filepath"
	"testing"
)

func TestProjectDir(t *testing.T) {
	root := t.TempDir()
	got := ProjectDir(root)
	want := filepath.Join(root, Name)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if ProjectRel(root) != Name {
		t.Fatal(ProjectRel(root))
	}
}

func TestJoinProject(t *testing.T) {
	root := t.TempDir()
	got := JoinProject(root, "goals", "x")
	want := filepath.Join(root, Name, "goals", "x")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
