package job

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSteerTakeAndFormat(t *testing.T) {
	if got := FormatSteerBlock("改用减法"); !strings.Contains(got, "[user_steering]") || !strings.Contains(got, "改用减法") {
		t.Fatalf("block=%q", got)
	}
	root := t.TempDir()
	m := New(func(context.Context, Job) (string, error) {
		return "run-1", nil
	}, nil)
	m.Bind(root)
	job, err := m.EnqueueGoal(GoalEnqueue{Objective: "do thing", Title: "g", SessionKey: QueueSessionKey("x"), MaxRounds: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Steer(job.ID, "先读 add.go"); err != nil {
		t.Fatal(err)
	}
	snap := m.Snapshot()
	var pending string
	for _, j := range snap.Jobs {
		if j.ID == job.ID {
			pending = j.SteerPending
		}
	}
	if pending != "先读 add.go" {
		t.Fatalf("pending=%q", pending)
	}
	got := m.TakeSteerPending(job.ID)
	if got != "先读 add.go" {
		t.Fatalf("take=%q", got)
	}
	if m.TakeSteerPending(job.ID) != "" {
		t.Fatal("expected empty after take")
	}
}

func TestSteerRunningDefault(t *testing.T) {
	root := t.TempDir()
	started := make(chan struct{})
	m := New(func(ctx context.Context, _ Job) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	}, nil)
	m.Bind(root)
	job, err := m.EnqueueSession("work", "", "t", "", "main", "", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = job
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout start")
	}
	out, err := m.Steer("", "转向")
	if err != nil {
		t.Fatal(err)
	}
	if out.SteerPending != "转向" {
		t.Fatalf("steer=%q", out.SteerPending)
	}
	_ = m.Cancel(out.ID)
}
