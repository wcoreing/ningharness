package toolgateway

import (
	"testing"

	"ningharness/workspace"
)

func TestProjectTurnAndFinishTurn(t *testing.T) {
	root := t.TempDir()
	ws := workspace.New()
	if _, err := ws.Open(root); err != nil {
		t.Fatal(err)
	}
	h := New(ws, nil)
	h.ArmTaskTrace(root, "t1", "main", "j1")
	h.ProjectTurn("t1", "main", "parent", "j1")
	tid, sess, parent := h.TurnContext()
	if tid != "t1" || sess != "main" || parent != "parent" || h.TurnJobID() != "j1" {
		t.Fatalf("proj=%s/%s/%s job=%s", tid, sess, parent, h.TurnJobID())
	}
	h.FinishTurn("ok", "")
	tid, sess, parent = h.TurnContext()
	if tid != "" || sess != "" || parent != "" || h.TurnJobID() != "" {
		t.Fatalf("expected clear, got %s/%s/%s job=%s", tid, sess, parent, h.TurnJobID())
	}
	if h.TaskTrace() != nil {
		t.Fatal("trace should be nil")
	}
	// 幂等
	h.FinishTurn("error", "x")
}

func TestDefaultsStyleOnExitOnlyFinish(t *testing.T) {
	// 模拟 BeginTask：投影 + OnExit Finish；EndTask 不再 Disarm
	root := t.TempDir()
	ws := workspace.New()
	if _, err := ws.Open(root); err != nil {
		t.Fatal(err)
	}
	h := New(ws, nil)
	h.ArmTaskTrace(root, "t2", "main", "")
	h.ProjectTurn("t2", "main", "", "job")
	// 成功路径只 Finish 一次
	h.FinishTurn("ok", "")
	if h.TaskTrace() != nil {
		t.Fatal("cleared")
	}
}
