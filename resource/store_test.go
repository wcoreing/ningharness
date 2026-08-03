package resource

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ningharness/deskdb"
)

func TestHistoryContentShortInline(t *testing.T) {
	got := HistoryContent(9, "get_skill", KindToolCall, "call", "ok", "", `{"skill":"extract"}`)
	if got != `{"skill":"extract"}` {
		t.Fatalf("short inline=%q", got)
	}
	if strings.Contains(got, "〔resource#") {
		t.Fatalf("should not tag inline content: %q", got)
	}
}

func TestPutGetSearch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitkeep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := deskdb.OpenProject(root); err != nil {
		t.Fatal(err)
	}

	longBody := strings.Repeat("甲", 600) + " 章节/十七.md 设定"
	id, summary, err := Put(root, PutInput{
		SessionKey: "main",
		TaskID:     "r1",
		ToolCallID: "c1",
		ToolName:   "read_file",
		Phase:      "result",
		RelPath:    "章节/十七.md",
		Body:       longBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Fatalf("id=%d", id)
	}
	if !strings.Contains(summary, "resource#") || !strings.Contains(summary, "recall_resource") {
		t.Fatalf("summary=%q", summary)
	}
	if strings.Contains(summary, longBody[:20]) {
		t.Fatalf("long body should not be fully inlined: %q", summary)
	}

	got, err := Get(root, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != longBody || got.RelPath != "章节/十七.md" {
		t.Fatalf("%+v", got)
	}

	text, err := Search(root, SearchOptions{ToolCallID: "c1", Limit: 5})
	if err != nil || !strings.Contains(text, "resource#") {
		t.Fatalf("search: %v %q", err, text)
	}

	diffID, diffSum, err := Put(root, PutInput{
		SessionKey: "main", TaskID: "r1", ToolCallID: "c1", ToolName: "write_file",
		Kind: KindDiff, RelPath: "a.md", Body: `{"path":"a.md","add":1,"del":0}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if diffID <= 0 || !strings.Contains(diffSum, "resource#") || !strings.Contains(diffSum, "diff") {
		t.Fatalf("diffSum=%q id=%d", diffSum, diffID)
	}
}
