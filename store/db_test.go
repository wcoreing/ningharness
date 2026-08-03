package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsEphemeralRoot(t *testing.T) {
	root := t.TempDir()
	if !isEphemeralRoot(root) {
		abs, _ := filepath.Abs(root)
		t.Fatalf("want ephemeral root=%q abs=%q tmp=%q", root, abs, os.TempDir())
	}
}

func TestOpenProjectEmpty(t *testing.T) {
	ResetCacheForTest()
	defer ResetCacheForTest()
	root := t.TempDir()
	db, err := OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	pid := ProjectID(root)
	if _, err := db.Exec(`INSERT INTO sessions(project_id, id, title, orch_key, updated_at_ms) VALUES(?,'main','main','o',1)`, pid); err != nil {
		t.Fatal(err)
	}
	var file string
	_ = db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&file)
	if file == "" || filepath.Base(file) != FileName {
		t.Fatalf("pragma file=%q", file)
	}
	if !strings.Contains(filepath.ToSlash(file), "/.agentdesk/") {
		t.Fatalf("want db under .agentdesk, got %q", file)
	}
}

func TestMigrateSessionsJSON(t *testing.T) {
	ResetCacheForTest()
	defer ResetCacheForTest()
	root := t.TempDir()
	ad := filepath.Join(root, ".agentdesk")
	if err := os.MkdirAll(ad, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `{"activeId":"main","sessions":[{"id":"main","title":"main","orchKey":"o","updatedAt":1,"messages":[{"role":"user","content":"hi","createdAt":1}]}]}`
	if err := os.WriteFile(filepath.Join(ad, "sessions.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	pid := ProjectID(root)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM history_message WHERE project_id=?`, pid).Scan(&n); err != nil || n != 1 {
		t.Fatalf("history_message=%d err=%v", n, err)
	}
	if fileExists(filepath.Join(ad, "sessions.json")) {
		t.Fatal("sessions.json should be removed")
	}
	var file string
	_ = db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&file)
	if !strings.Contains(filepath.ToSlash(file), "/.agentdesk/") {
		t.Fatalf("want db under .agentdesk, got %q", file)
	}
}

func TestSingleDBPath(t *testing.T) {
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != FileName {
		t.Fatal(p)
	}
}
