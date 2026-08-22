package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMountBriefWithScripts(t *testing.T) {
	root := t.TempDir()
	id := "demo-script"
	base := filepath.Join(Dir(root), id, "scripts")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "run.sh"), []byte("#!/bin/sh\necho hi"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := MountBrief(root, []string{id})
	if got == "" || !strings.Contains(got, "get_skill demo-script") || !strings.Contains(got, "read_file") {
		t.Fatalf("MountBrief: %q", got)
	}
}

func TestMountBriefEmpty(t *testing.T) {
	if MountBrief("", nil) != "" {
		t.Fatal("expected empty")
	}
}
