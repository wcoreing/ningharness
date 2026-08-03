package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenameAcceptsPathInNewName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "章节")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "第三章_炉中山海.md")
	if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New()
	if _, err := s.Open(root); err != nil {
		t.Fatal(err)
	}
	res, err := s.Rename("章节/第三章_炉中山海.md", "章节/第三章_兰若寺.md", "test")
	if err != nil {
		t.Fatal(err)
	}
	want := "章节/第三章_兰若寺.md"
	if res.MovedTo["章节/第三章_炉中山海.md"] != want {
		t.Fatalf("movedTo=%v want %s", res.MovedTo, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "第三章_兰若寺.md")); err != nil {
		t.Fatal(err)
	}
}
