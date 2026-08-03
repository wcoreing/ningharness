package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateListLoad(t *testing.T) {
	root := t.TempDir()
	info, err := Create(root, "demo", "demo", "demo skill", "Do the thing.")
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "demo" || info.Name != "demo" {
		t.Fatalf("info=%+v", info)
	}
	list, err := List(root)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	_, body, err := LoadBody(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Do the thing") {
		t.Fatalf("body=%q", body)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "system", "skills", "demo", "SKILL.md"))
	if !strings.Contains(string(raw), "name: demo") {
		t.Fatalf("skill md=%q", raw)
	}
	if _, err := Delete(root, "demo"); err != nil {
		t.Fatal(err)
	}
	list2, err := List(root)
	if err != nil || len(list2) != 0 {
		t.Fatalf("after delete list=%v err=%v", list2, err)
	}
}

func TestInvalidID(t *testing.T) {
	_, err := Create(t.TempDir(), "../x", "n", "d", "")
	if err == nil {
		t.Fatal("expected error")
	}
}
