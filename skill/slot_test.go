package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiskSlot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, RootDir, "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: Demo\ndescription: d\nglobs:\n  - user/**\n---\n\n# body\n"
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewDisk()
	list, err := s.List(root)
	if err != nil || len(list) != 1 || list[0].ID != "demo" {
		t.Fatalf("List=%v err=%v", list, err)
	}
	matched := s.Match(root, []string{"user/a.md"})
	if len(matched) != 1 {
		t.Fatalf("Match=%v", matched)
	}
	info, body, err := s.Load(root, "demo")
	if err != nil || info.ID != "demo" || body == "" {
		t.Fatalf("Load info=%+v body=%q err=%v", info, body, err)
	}
	ids := IDs(matched)
	if len(ids) != 1 || ids[0] != "demo" {
		t.Fatalf("IDs=%v", ids)
	}
	paths := PathsFromValues(map[string]any{PathsValueKey: []string{" user/a.md ", ""}})
	if len(paths) != 1 || paths[0] != "user/a.md" {
		t.Fatalf("paths=%v", paths)
	}
}
