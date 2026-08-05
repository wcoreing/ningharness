package toolgateway

import (
	"os"
	"testing"

	"ningharness/trace"
	"ningharness/workspace"
)

func TestWithTaskTraceWritesJSONL(t *testing.T) {
	root := t.TempDir()
	ws := workspace.New()
	if _, err := ws.Open(root); err != nil {
		t.Fatal(err)
	}
	h := New(ws, nil)
	err := h.WithTaskTrace(root, "task-a", "main", "job-1", func() error {
		if h.TaskTrace() == nil {
			t.Fatal("trace not armed")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.TaskTrace() != nil {
		t.Fatal("trace not cleared")
	}
	path, err := trace.FindByTaskID(root, "task-a")
	if err != nil {
		t.Fatal(err)
	}
	st, evs, err := trace.InspectFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Complete || st.EndedStatus != "ok" {
		t.Fatalf("resume=%+v", st)
	}
	if len(evs) < 2 {
		t.Fatalf("events=%d", len(evs))
	}
}

func TestArmDisarmTaskTrace(t *testing.T) {
	root := t.TempDir()
	ws := workspace.New()
	if _, err := ws.Open(root); err != nil {
		t.Fatal(err)
	}
	h := New(ws, nil)
	h.ArmTaskTrace(root, "task-b", "sess", "j")
	if h.TaskTrace() == nil {
		t.Fatal("expected writer")
	}
	h.DisarmTaskTrace("ok", "")
	if h.TaskTrace() != nil {
		t.Fatal("expected clear")
	}
	p, err := trace.FindByTaskID(root, "task-b")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}
