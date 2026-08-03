package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ningharness/deskdb"
	"ningharness/history"
)

func TestSaveGetList(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".gitkeep"), []byte(""), 0o644)
	if _, err := deskdb.OpenProject(root); err != nil {
		t.Fatal(err)
	}
	if err := history.Append(root, "main", history.Msg{Role: "thinking", Content: "hi", TaskID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if err := history.Append(root, "main", history.Msg{
		Role: "assistant", TaskID: "t1",
		ToolCallsJSON: history.EncodeToolCalls([]history.ToolCallSpec{
			{ID: "c1", Name: "write_file", Arguments: `{"rel_path":"a.md"}`},
		}),
	}); err != nil {
		t.Fatal(err)
	}
	if err := history.Append(root, "main", history.Msg{
		Role: "tool", ToolCallID: "c1", Content: "ok", TaskID: "t1",
	}); err != nil {
		t.Fatal(err)
	}
	rec := Record{
		ID: "t1", Driver: "wnai", Status: "ok", JobID: "job-1", SessionID: "main",
		Tools: []ToolCall{{Name: "write_file", OK: true, Path: "a.md"}},
	}
	if err := Save(root, rec); err != nil {
		t.Fatal(err)
	}
	got, err := Get(root, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if got.JobID != "job-1" || len(got.Steps) != 2 || len(got.Tools) != 1 {
		t.Fatalf("%+v", got)
	}
	list, err := List(root, 10, true)
	if err != nil || len(list) != 1 || list[0].ToolCount != 1 {
		t.Fatalf("%v %+v", err, list)
	}
}

func TestGuessRelPath(t *testing.T) {
	if g := GuessRelPath(`{"path":"src/a.md"}`); g != "src/a.md" {
		t.Fatal(g)
	}
	if g := GuessRelPath("Successfully wrote draft '章节/第十一章.md'（1024 字）"); g != "章节/第十一章.md" {
		t.Fatalf("wrote draft=%q", g)
	}
	if g := GuessRelPath("Successfully edited 'user/草案/a.md'（1 处 · 3 字）"); g != "user/草案/a.md" {
		t.Fatal(g)
	}
}

func TestSaveGetFeedforwardFromHistory(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".gitkeep"), []byte(""), 0o644)
	if _, err := deskdb.OpenProject(root); err != nil {
		t.Fatal(err)
	}
	ff := "## 项目现状\n- focus: a.md"
	if err := history.Append(root, "main", history.Msg{
		Role: "user", Content: "续写", Feedforward: ff, TaskID: "t-ff",
	}); err != nil {
		t.Fatal(err)
	}
	if err := Save(root, Record{ID: "t-ff", Driver: "wnai", Status: "ok", SessionID: "main"}); err != nil {
		t.Fatal(err)
	}
	got, err := Get(root, "t-ff")
	if err != nil {
		t.Fatal(err)
	}
	if got.Feedforward != ff {
		t.Fatalf("Feedforward=%q", got.Feedforward)
	}
	sum := FormatSummary(got)
	if !strings.Contains(sum, "feedforward:") {
		t.Fatalf("summary=%s", sum)
	}
}

