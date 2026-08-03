package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnabledDefaultAndFilter(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(root, "on", "on", "enabled by default", "body"); err != nil {
		t.Fatal(err)
	}
	offDir := filepath.Join(Dir(root), "off")
	if err := os.MkdirAll(offDir, 0o755); err != nil {
		t.Fatal(err)
	}
	offMD := "---\nname: off\ndescription: disabled\nenabled: false\n---\n\n# off\n"
	if err := os.WriteFile(filepath.Join(offDir, SkillFile), []byte(offMD), 0o644); err != nil {
		t.Fatal(err)
	}

	all, err := List(root)
	if err != nil || len(all) != 2 {
		t.Fatalf("list all=%v err=%v", all, err)
	}
	var onInfo, offInfo Info
	for _, s := range all {
		switch s.ID {
		case "on":
			onInfo = s
		case "off":
			offInfo = s
		}
	}
	if !onInfo.Enabled {
		t.Fatalf("default enabled want true: %+v", onInfo)
	}
	if offInfo.Enabled {
		t.Fatalf("enabled:false want false: %+v", offInfo)
	}

	en, err := ListEnabled(root)
	if err != nil || len(en) != 1 || en[0].ID != "on" {
		t.Fatalf("ListEnabled=%v err=%v", en, err)
	}
}

func TestRenderSetEnabled(t *testing.T) {
	root := t.TempDir()
	if _, err := Create(root, "demo", "demo", "d", "body text"); err != nil {
		t.Fatal(err)
	}
	rel, doc, info, err := RenderSetEnabled(root, "demo", false)
	if err != nil {
		t.Fatal(err)
	}
	if rel != "system/skills/demo/SKILL.md" || info.Enabled {
		t.Fatalf("rel=%q info=%+v", rel, info)
	}
	if !strings.Contains(doc, "enabled: false") || !strings.Contains(doc, "body text") {
		t.Fatalf("doc=%q", doc)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	rel2, doc2, info2, err := RenderSetEnabled(root, "demo", true)
	if err != nil {
		t.Fatal(err)
	}
	if !info2.Enabled || strings.Contains(doc2, "enabled:") {
		t.Fatalf("re-enable should omit key: info=%+v doc=%q", info2, doc2)
	}
	_ = rel2
}
