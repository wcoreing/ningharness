package job

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ningharness/store"
	agenttask "ningharness/task"
)

func TestEnqueueIsolatedSessionKey(t *testing.T) {
	root := t.TempDir()
	m := New(func(ctx context.Context, task Job) (string, error) {
		return "r", nil
	}, nil)
	m.Bind(root)
	if err := m.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	job, err := m.EnqueueIsolated("smoke", "", "iso", "章节/a.md")
	if err != nil {
		t.Fatal(err)
	}
	want := QueueSessionKey(job.ID)
	if job.SessionKey != want {
		t.Fatalf("SessionKey=%q want %q", job.SessionKey, want)
	}
	m.Shutdown()
}

func TestEnqueuePathsAndSerialRun(t *testing.T) {
	root := t.TempDir()
	var n atomic.Int32
	var sawRels []string
	m := New(func(ctx context.Context, task Job) (string, error) {
		n.Add(1)
		if task.TargetRel == "" {
			t.Fatalf("empty target")
		}
		if strings.Contains(task.Prompt, task.TargetRel) {
			t.Fatalf("path must not be injected into prompt: %q", task.Prompt)
		}
		if !strings.HasPrefix(task.SessionKey, "once:queue:") {
			t.Fatalf("sessionKey=%q", task.SessionKey)
		}
		sawRels = append(sawRels, task.TargetRel)
		abs := filepath.Join(root, filepath.FromSlash(task.TargetRel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("# ok\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return "run-x", nil
	}, nil)
	m.Bind(root)
	tasks, err := m.EnqueuePaths([]string{"user/草案/a.md", "user/草案/b.md"}, "", "wnai", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1 batch task, got %d", len(tasks))
	}
	if len(tasks[0].Steps) != 2 {
		t.Fatalf("steps=%d", len(tasks[0].Steps))
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s := m.Snapshot()
		if s.Stats.Done == 1 && s.Stats.Queued == 0 && s.Stats.Running == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	s := m.Snapshot()
	if s.Stats.Done != 1 {
		t.Fatalf("stats=%+v n=%d", s.Stats, n.Load())
	}
	if n.Load() != 2 {
		t.Fatalf("exec count=%d want 2 serial steps", n.Load())
	}
	if len(sawRels) != 2 || sawRels[0] != "user/草案/a.md" || sawRels[1] != "user/草案/b.md" {
		t.Fatalf("rels=%v", sawRels)
	}
	var batch Job
	for _, tsk := range s.Jobs {
		if tsk.ID == tasks[0].ID {
			batch = tsk
			break
		}
	}
	if batch.StepDone != 2 || batch.StepTotal != 2 {
		t.Fatalf("progress done=%d total=%d hint=%q", batch.StepDone, batch.StepTotal, batch.ProgressHint)
	}
	if _, err := store.OpenProject(root); err != nil {
		t.Fatal(err)
	}
	m.Shutdown()
}

func TestAgentTurnNoWriteFileStillDone(t *testing.T) {
	root := t.TempDir()
	rel := "章节/第三章.md"
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(func(ctx context.Context, task Job) (string, error) {
		runID := "run-empty"
		if err := agenttask.Save(root, agenttask.Record{ID: runID, Status: "ok"}); err != nil {
			t.Fatal(err)
		}
		return runID, nil
	}, nil)
	m.Bind(root)
	task, err := m.Enqueue("第十章写多少字", "wnai", "", rel)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	var got Job
	for time.Now().Before(deadline) {
		for _, tsk := range m.Snapshot().Jobs {
			if tsk.ID == task.ID {
				got = tsk
				if tsk.Status == StatusError || tsk.Status == StatusDone {
					break
				}
			}
		}
		if got.Status == StatusError || got.Status == StatusDone {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got.Status != StatusDone {
		t.Fatalf("want done without write gate, got status=%s err=%q", got.Status, got.Error)
	}
	m.Shutdown()
}

func TestCancelQueued(t *testing.T) {
	root := t.TempDir()
	m := New(func(ctx context.Context, task Job) (string, error) {
		return "r", nil
	}, nil)
	m.Bind(root)
	t1, err := m.Enqueue("one", "", "t1", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Enqueue("two", "", "t2", ""); err != nil {
		t.Fatal(err)
	}
	// 入队会自动继续；测取消前先暂停，避免立刻跑完
	if err := m.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	if err := m.Cancel(t1.ID); err != nil {
		t.Fatal(err)
	}
	s := m.Snapshot()
	if s.Stats.Cancelled != 1 || s.Stats.Queued != 1 {
		t.Fatalf("stats=%+v", s.Stats)
	}
	m.Shutdown()
}

func TestEnqueueResumesPaused(t *testing.T) {
	root := t.TempDir()
	var n atomic.Int32
	m := New(func(ctx context.Context, task Job) (string, error) {
		n.Add(1)
		return "r", nil
	}, nil)
	m.Bind(root)
	if err := m.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Enqueue("go", "", "t", ""); err != nil {
		t.Fatal(err)
	}
	if m.Snapshot().Paused {
		t.Fatal("enqueue should clear pause")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if n.Load() >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if n.Load() < 1 {
		t.Fatal("expected exec after enqueue resume")
	}
	m.Shutdown()
}

func TestDefaultPathPromptNoPathInjection(t *testing.T) {
	if strings.Contains(DefaultPathPrompt, "{path}") {
		t.Fatal("default template must not embed path placeholder")
	}
}

func TestMoveBeforeQueued(t *testing.T) {
	root := t.TempDir()
	started := make(chan struct{}, 8)
	release := make(chan struct{})
	m := New(func(ctx context.Context, task Job) (string, error) {
		started <- struct{}{}
		select {
		case <-release:
			return "r", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}, nil)
	m.SetMaxParallel(1)
	m.Bind(root)
	a, err := m.Enqueue("a", "", "a", "a.md")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("a should start")
	}
	b, err := m.Enqueue("b", "", "b", "b.md")
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.Enqueue("c", "", "c", "c.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SetPaused(true); err != nil {
		t.Fatal(err)
	}
	// a running；b、c queued → 调序 b/c
	if err := m.MoveBefore(c.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	s := m.Snapshot()
	var ids []string
	for _, tsk := range s.Jobs {
		if tsk.Status == StatusQueued {
			ids = append(ids, tsk.ID)
		}
	}
	if len(ids) != 2 || ids[0] != c.ID || ids[1] != b.ID {
		t.Fatalf("queued order=%v want c,b (a running)", ids)
	}
	if err := m.MoveByDir(b.ID, "up"); err != nil {
		t.Fatal(err)
	}
	s = m.Snapshot()
	ids = ids[:0]
	for _, tsk := range s.Jobs {
		if tsk.Status == StatusQueued {
			ids = append(ids, tsk.ID)
		}
	}
	// c, b → b up → b, c
	if len(ids) != 2 || ids[0] != b.ID || ids[1] != c.ID {
		t.Fatalf("after up order=%v", ids)
	}
	_ = a
	m.CancelRunning()
	close(release)
	m.Shutdown()
	time.Sleep(50 * time.Millisecond)
}

func TestParallelRuns(t *testing.T) {
	root := t.TempDir()
	var peak atomic.Int32
	var cur atomic.Int32
	gate := make(chan struct{})
	m := New(func(ctx context.Context, task Job) (string, error) {
		n := cur.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		<-gate
		cur.Add(-1)
		return "run-" + task.ID, nil
	}, nil)
	m.SetMaxParallel(2)
	m.Bind(root)
	if _, err := m.Enqueue("a", "", "a", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Enqueue("b", "", "b", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Enqueue("c", "", "c", ""); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if peak.Load() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if peak.Load() < 2 {
		close(gate)
		m.Shutdown()
		t.Fatalf("expected peak concurrency >=2, got %d", peak.Load())
	}
	close(gate)
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s := m.Snapshot()
		if s.Stats.Done == 3 && s.Stats.Running == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	s := m.Snapshot()
	if s.Stats.Done != 3 || s.MaxParallel != 2 {
		t.Fatalf("stats=%+v maxParallel=%d", s.Stats, s.MaxParallel)
	}
	m.Shutdown()
}

func TestRetryFailedClearsPauseAndRuns(t *testing.T) {
	root := t.TempDir()
	var n atomic.Int32
	var sawRetry atomic.Bool
	m := New(func(ctx context.Context, task Job) (string, error) {
		n.Add(1)
		if task.RetryCount > 0 {
			sawRetry.Store(true)
			if !strings.Contains(WrapRetryPrompt(task), "失败重启") {
				t.Fatal("expected retry envelope")
			}
		}
		if n.Load() == 1 {
			return "", fmt.Errorf("boom")
		}
		return "ok", nil
	}, nil)
	m.Bind(root)
	if err := m.SetPauseOnError(true); err != nil {
		t.Fatal(err)
	}
	first, err := m.Enqueue("fail-me", "wnai", "bad", "")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s := m.Snapshot()
		if s.Stats.Error == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	s := m.Snapshot()
	if s.Stats.Error != 1 || !s.Paused {
		t.Fatalf("want 1 error + paused, got %+v paused=%v", s.Stats, s.Paused)
	}
	count, err := m.RetryFailed()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("retried=%d", count)
	}
	s = m.Snapshot()
	if s.Paused {
		t.Fatal("retry should clear pause")
	}
	if s.Stats.Queued != 1 {
		t.Fatalf("want same task re-queued, stats=%+v", s.Stats)
	}
	found := false
	for _, tsk := range s.Jobs {
		if tsk.ID == first.ID {
			found = true
			if tsk.RetryCount != 1 || tsk.LastError == "" {
				t.Fatalf("task=%+v", tsk)
			}
		}
	}
	if !found {
		t.Fatal("expected same task id after retry")
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s = m.Snapshot()
		if s.Stats.Done >= 1 && s.Stats.Queued == 0 && s.Stats.Running == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	s = m.Snapshot()
	if s.Stats.Done < 1 {
		t.Fatalf("expected retry success, stats=%+v n=%d", s.Stats, n.Load())
	}
	if !sawRetry.Load() {
		t.Fatal("executor should see RetryCount>0")
	}
	m.Shutdown()
}

func TestWrapRetryPrompt(t *testing.T) {
	p := WrapRetryPrompt(Job{
		Prompt:     "写第三章",
		TargetRel:  "user/草案/3.md",
		LastError:  "args: unexpected end of JSON input",
		RetryCount: 2,
	})
	if !strings.Contains(p, "失败重启") || !strings.Contains(p, "read_file") {
		t.Fatalf("envelope=%q", p)
	}
	if !strings.Contains(p, "user/草案/3.md") || !strings.Contains(p, "写第三章") {
		t.Fatalf("envelope=%q", p)
	}
	if !strings.Contains(p, "unexpected end of JSON") {
		t.Fatalf("missing last error: %q", p)
	}
}
