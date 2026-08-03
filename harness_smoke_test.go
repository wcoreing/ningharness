package ningharness_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ningharness"
	"ningharness/store"
	"ningharness/job"
	"ningharness/lesson"
	"ningharness/task"
)

func TestHarnessSmoke(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	runID := "smoke-run-1"
	h, err := ningharness.Open(ningharness.Opts{DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	h.Job = job.New(func(ctx context.Context, j job.Job) (string, error) {
		return runID, nil
	}, nil)

	root := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := h.UseProject(root); err != nil {
		t.Fatal(err)
	}
	pid := store.ProjectID(root)

	if _, err := h.Session.Append(root, pid, "main", "user", "smoke hello", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := task.Save(root, task.Record{ID: runID, Status: "ok"}); err != nil {
		t.Fatal(err)
	}

	queued, err := h.Job.Enqueue("smoke job", "", "smoke-key", "")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s := h.Job.Snapshot()
		if s.Stats.Done >= 1 && s.Stats.Running == 0 {
			break
		}
		for _, j := range s.Jobs {
			if j.ID == queued.ID && (j.Status == job.StatusDone || j.Status == job.StatusError) {
				goto doneWait
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
doneWait:
	s := h.Job.Snapshot()
	if s.Stats.Done < 1 {
		t.Fatalf("expected job done, stats=%+v", s.Stats)
	}

	if _, err := lesson.Append(lesson.AppendInput{
		Scope: lesson.ScopePersonal,
		Body:  "smoke personal lesson",
	}); err != nil {
		t.Fatal(err)
	}

	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
}
