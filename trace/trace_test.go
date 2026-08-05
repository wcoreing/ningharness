package trace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBeginToolPairEnd(t *testing.T) {
	root := t.TempDir()
	w, err := Begin(root, "t1", "main", "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ToolCall("c1", "read_file", `{"rel_path":"a.md"}`); err != nil {
		t.Fatal(err)
	}
	if err := w.ToolResult("c1", "read_file", "ok body", true); err != nil {
		t.Fatal(err)
	}
	if err := w.End("ok", ""); err != nil {
		t.Fatal(err)
	}
	st, evs, err := InspectFile(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Complete || st.EndedStatus != "ok" || len(st.UnpairedCalls) != 0 {
		t.Fatalf("st=%+v", st)
	}
	if len(evs) != 4 {
		t.Fatalf("events=%d", len(evs))
	}
	found, err := FindByTaskID(root, "t1")
	if err != nil || found != w.Path() {
		t.Fatalf("find=%q err=%v", found, err)
	}
}

func TestResumeUnpairedAndTruncatedLine(t *testing.T) {
	root := t.TempDir()
	w, err := Begin(root, "t2", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ToolCall("c9", "write_file", "huge"); err != nil {
		t.Fatal(err)
	}
	// 截断行
	f, err := os.OpenFile(w.Path(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"type":"tool_result","tool_call_id":"c9"`) // incomplete
	_ = f.Close()

	st, _, err := InspectFile(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	if st.Complete || len(st.UnpairedCalls) != 1 || st.UnpairedCalls[0] != "c9" {
		t.Fatalf("st=%+v", st)
	}
}

func TestAbortKeepsIncomplete(t *testing.T) {
	root := t.TempDir()
	w, err := Begin(root, "t3", "", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = w.ToolCall("c1", "grep", "x")
	_ = w.Abort("cancelled")
	st, _, err := InspectFile(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	if st.Complete {
		t.Fatal("abort must not complete")
	}
}

func TestBriefTrim(t *testing.T) {
	root := t.TempDir()
	w, err := Begin(root, "t4", "", "")
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("字", maxBriefRunes+20)
	_ = w.ToolCall("c1", "write_file", long)
	evs, err := Load(w.Path())
	if err != nil {
		t.Fatal(err)
	}
	var brief string
	for _, e := range evs {
		if e.Type == TypeToolCall {
			brief = e.ArgsBrief
		}
	}
	if !strings.HasSuffix(brief, "…") {
		t.Fatalf("brief=%q", brief)
	}
	_ = filepath.Walk(Dir(root), func(path string, info os.FileInfo, err error) error { return nil })
}
