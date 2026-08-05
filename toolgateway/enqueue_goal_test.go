package toolgateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	deskqueue "ningharness/job"
	"ningharness/session"
	"ningharness/workspace"
)

func TestInvokeEnqueueGoal(t *testing.T) {
	root := t.TempDir()
	ws := workspace.New()
	if _, err := ws.Open(root); err != nil {
		t.Fatal(err)
	}
	h := New(ws, session.NewStore())
	var sawPrompt string
	m := deskqueue.New(func(ctx context.Context, job deskqueue.Job) (string, error) {
		sawPrompt = job.WirePrompt
		if sawPrompt == "" {
			sawPrompt = job.Prompt
		}
		control := filepath.Join(root, ".ningharness", "goals", job.ID, "GOAL.yaml")
		_ = os.WriteFile(control, []byte("objective: x\nstatus: complete\n"), 0o644)
		return "r", nil
	}, nil)
	m.Bind(root)
	h.Queue = m

	out, err := h.Invoke(context.Background(), "enqueue_goal", `{"objective":"ship feature","max_rounds":5,"session":"isolated"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "已入队") || !strings.Contains(out, "type=goal") {
		t.Fatalf("out=%s", out)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s := m.Snapshot()
		if s.Stats.Done == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if m.Snapshot().Stats.Done != 1 {
		t.Fatalf("stats=%+v prompt=%q", m.Snapshot().Stats, sawPrompt)
	}
	if !strings.Contains(sawPrompt, "[goal]") {
		t.Fatalf("wire=%q", sawPrompt)
	}
	m.Shutdown()
}
