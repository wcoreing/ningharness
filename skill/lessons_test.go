package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBodyOmitsLessonsFile(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(root, "demo", "demo", "d", "skill body"); err != nil {
		t.Fatal(err)
	}
	lessons := filepath.Join(root, "system", "skills", "demo", "LESSONS.md")
	body := "# LESSONS\n\n## 2026-01-01 · superseded\n\nold way\n\n## 2026-02-01 · active\n\nnew way\n"
	if err := os.WriteFile(lessons, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, got, err := LoadBody(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "old way") || strings.Contains(got, "LESSONS") {
		t.Fatalf("LoadBody must not attach LESSONS.md: %s", got)
	}
	if !strings.Contains(got, "skill body") {
		t.Fatalf("want skill body: %s", got)
	}
}
