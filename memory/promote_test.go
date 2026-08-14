package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wcoreing/ningharness/lesson"
	"github.com/wcoreing/ningharness/store"
)

func TestToolErrorPromoter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	store.ResetCacheForTest()
	t.Cleanup(store.ResetCacheForTest)
	if _, err := store.OpenProject(root); err != nil {
		t.Fatal(err)
	}
	p := ToolErrorPromoter{}
	notes := NotesFromToolErrors([]string{"edit: old_string not found", "write_file: path denied"})
	if err := p.Ingest(context.Background(), IngestInput{
		Root: root, TaskID: "task-1", Prompt: "fix chapter", Notes: notes,
	}); err != nil {
		t.Fatal(err)
	}
	list, err := lesson.ListByScope(root, lesson.ScopeProject, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("lessons=%d", len(list))
	}
	if !strings.Contains(list[0].Body, "[tool_error]") {
		t.Fatalf("body=%q", list[0].Body)
	}
	if err := p.Ingest(context.Background(), IngestInput{Root: root, Notes: notes}); err != nil {
		t.Fatal(err)
	}
	list2, err := lesson.ListByScope(root, lesson.ScopeProject, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list2) != 2 {
		t.Fatalf("dedup failed lessons=%d", len(list2))
	}
}

func TestPromoteGoalBlockedDedup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	store.ResetCacheForTest()
	t.Cleanup(store.ResetCacheForTest)
	if _, err := store.OpenProject(root); err != nil {
		t.Fatal(err)
	}
	if err := PromoteGoalBlocked(root, "need key", "ship", "t1", "main"); err != nil {
		t.Fatal(err)
	}
	if err := PromoteGoalBlocked(root, "need key", "ship", "t2", "main"); err != nil {
		t.Fatal(err)
	}
	list, err := lesson.ListActiveProject(root, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 lesson, got %d", len(list))
	}
}

func TestParseToolErrorNotes(t *testing.T) {
	got := parseToolErrorNotes(NotesFromToolErrors([]string{"a", "b"}))
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("%v", got)
	}
	if parseToolErrorNotes("") != nil {
		t.Fatal("empty")
	}
}
