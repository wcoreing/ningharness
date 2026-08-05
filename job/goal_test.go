package job

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func waitJobStatus(t *testing.T, m *Manager, id string, want Status, timeout time.Duration) Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s := m.Snapshot()
		for _, j := range s.Jobs {
			if j.ID == id && j.Status == want {
				return j
			}
		}
		for _, j := range s.Jobs {
			if j.ID == id && (j.Status == StatusDone || j.Status == StatusError || j.Status == StatusCancelled) {
				if j.Status != want {
					t.Fatalf("job %s status=%s want=%s err=%q hint=%q", id, j.Status, want, j.Error, j.ProgressHint)
				}
				return j
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting job %s -> %s", id, want)
	return Job{}
}

func TestEnqueueGoalAndRun(t *testing.T) {
	root := t.TempDir()
	var rounds atomic.Int32
	m := New(func(ctx context.Context, task Job) (string, error) {
		n := int(rounds.Add(1))
		if !strings.Contains(task.WirePrompt, "[goal]") && !strings.Contains(task.Prompt, "[goal]") {
			t.Fatalf("missing goal prompt: wire=%q prompt=%q", task.WirePrompt, task.Prompt)
		}
		control := filepath.Join(root, ".ningharness", "goals", task.ID, "GOAL.yaml")
		if n < 2 {
			return "r1", nil
		}
		if err := os.WriteFile(control, []byte("objective: ship\nstatus: complete\n"), 0o644); err != nil {
			return "", err
		}
		return "r2", nil
	}, nil)
	m.Bind(root)
	job, err := m.EnqueueGoal("ship", "", "g", "", "", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if job.Type != JobTypeGoal || job.GoalMaxRounds != 10 {
		t.Fatalf("job=%+v", job)
	}
	done := waitJobStatus(t, m, job.ID, StatusDone, 3*time.Second)
	if done.GoalRound != 2 {
		t.Fatalf("GoalRound=%d", done.GoalRound)
	}
	if !strings.Contains(done.ProgressHint, "完成") {
		t.Fatalf("hint=%q", done.ProgressHint)
	}
	if rounds.Load() != 2 {
		t.Fatalf("rounds=%d", rounds.Load())
	}
	m.Shutdown()
}

func TestEnqueueGoalMaxRounds(t *testing.T) {
	root := t.TempDir()
	m := New(func(ctx context.Context, task Job) (string, error) {
		return "r", nil
	}, nil)
	m.Bind(root)
	job, err := m.EnqueueGoal("never", "", "g", "", "", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	done := waitJobStatus(t, m, job.ID, StatusError, 3*time.Second)
	if done.GoalRound != 2 {
		t.Fatalf("GoalRound=%d", done.GoalRound)
	}
	if !strings.Contains(done.Error, "max rounds") {
		t.Fatalf("error=%q", done.Error)
	}
	m.Shutdown()
}

func TestEnqueueGoalCancel(t *testing.T) {
	root := t.TempDir()
	started := make(chan struct{}, 1)
	m := New(func(ctx context.Context, task Job) (string, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return "", ctx.Err()
	}, nil)
	m.Bind(root)
	job, err := m.EnqueueGoal("abort", "", "g", "", "", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("exec should start")
	}
	if err := m.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	_ = waitJobStatus(t, m, job.ID, StatusCancelled, 3*time.Second)
	m.Shutdown()
}
