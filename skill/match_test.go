package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchPathPrefix(t *testing.T) {
	if !MatchPath("user/草案/**", "user/草案/01-商务/1.1.md") {
		t.Fatal("prefix /**")
	}
	if MatchPath("user/草案/**", "user/查阅/x.md") {
		t.Fatal("should not match")
	}
}

func TestMatchPathDoublestarBase(t *testing.T) {
	if !MatchPath("**/1.1-*.md", "user/草案/01-商务/1.1-资质.md") {
		t.Fatal("**/base")
	}
}

func TestMatchForPaths(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "system", "skills", "w")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "---\nname: w\ndescription: d\nglobs:\n  - user/草案/**\n---\n\n# w\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	got := MatchForPaths(root, []string{"user/草案/x.md"})
	if len(got) != 1 || got[0].ID != "w" {
		t.Fatalf("%#v", got)
	}
	if len(MatchForPaths(root, []string{"other/a.md"})) != 0 {
		t.Fatal("no match expected")
	}
}

func TestMatchForPathsSkipsDisabled(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "system", "skills", "w")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "---\nname: w\ndescription: d\nenabled: false\nglobs:\n  - user/草案/**\n---\n\n# w\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(MatchForPaths(root, []string{"user/草案/x.md"})) != 0 {
		t.Fatal("disabled skill must not path-match")
	}
}


