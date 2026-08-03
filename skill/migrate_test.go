package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateRootIfNeeded(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "skills", "writing")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "SKILL.md"), []byte("# w\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mig, err := MigrateRootIfNeeded(root)
	if err != nil {
		t.Fatal(err)
	}
	if !mig {
		t.Fatal("expected migrated")
	}
	neo := filepath.Join(root, "system", "skills", "writing", "SKILL.md")
	if _, err := os.Stat(neo); err != nil {
		t.Fatalf("missing new skill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills")); !os.IsNotExist(err) {
		t.Fatalf("legacy should be gone: %v", err)
	}
	mig2, err := MigrateRootIfNeeded(root)
	if err != nil || mig2 {
		t.Fatalf("second migrate: mig=%v err=%v", mig2, err)
	}
}

func TestMigrateRootIfNeededBothExist(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "skills", "a"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "skills", "a", "SKILL.md"), []byte("old\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(root, "system", "skills", "b"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "system", "skills", "b", "SKILL.md"), []byte("new\n"), 0o644)
	mig, err := MigrateRootIfNeeded(root)
	if err != nil {
		t.Fatal(err)
	}
	if mig {
		t.Fatal("should not migrate when both exist")
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "a", "SKILL.md")); err != nil {
		t.Fatal("legacy should remain")
	}
}
