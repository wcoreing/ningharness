package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileIngestAndBundle(t *testing.T) {
	root := t.TempDir()
	b := NewLessonWithFileIngest()
	if err := b.Ingest(context.Background(), IngestInput{
		Root: root, SessionKey: "main", TaskID: "t1",
		Prompt: "p", Reply: "r", SkillIDs: []string{"demo"},
	}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, defaultIngestRelDir)
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) != 1 {
		t.Fatalf("dir=%v err=%v", ents, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ents[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"taskId":"t1"`) || !strings.Contains(string(raw), `"reply":"r"`) {
		t.Fatalf("%s", raw)
	}
	if err := b.Ingest(context.Background(), IngestInput{
		Root: root, SessionKey: "main", TaskID: "t2",
		Prompt: "p2", Reply: "r2", Notes: "tool_errors:\n- terminal.exec: bad",
	}); err != nil {
		t.Fatal(err)
	}
	raw2, err := os.ReadFile(filepath.Join(dir, ents[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw2), `"notes":"tool_errors`) {
		t.Fatalf("notes missing: %s", raw2)
	}
	patch, err := b.Assemble(context.Background(), AssembleInput{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	_ = patch
}
