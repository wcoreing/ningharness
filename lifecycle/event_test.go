package lifecycle

import (
	"context"
	"errors"
	"testing"
)

func TestBusGateBlockAndWatchIgnoredError(t *testing.T) {
	var log []string
	ran := false
	wf := New(Step{ID: StepRunGuest, Run: func(ctx context.Context, st *RunState) error {
		ran = true
		st.Reply = "x"
		return nil
	}})
	wf.Watch(StepRunGuest, Before, func(ctx context.Context, ev Event) error {
		log = append(log, "watch-before")
		return errors.New("watch noise")
	})
	wf.On(StepRunGuest, Before, func(ctx context.Context, ev Event) error {
		log = append(log, "gate-before")
		return ErrBlock
	})
	wf.Watch(StepRunGuest, After, func(ctx context.Context, ev Event) error {
		log = append(log, "watch-after")
		return nil
	})

	err := (Runner{}).Run(context.Background(), wf, &RunState{})
	if !errors.Is(err, ErrBlock) {
		t.Fatalf("err=%v", err)
	}
	if ran {
		t.Fatal("run should not execute")
	}
	// Gate runs before Watch for same moment? Emit: gates first, then watches.
	// On Block during gate, watches for that moment are NOT run.
	if len(log) != 1 || log[0] != "gate-before" {
		t.Fatalf("log=%v (watch-before should not run after gate block; gates run first)", log)
	}
}

func TestBusWatchSeesAfterState(t *testing.T) {
	var afterReply string
	wf := New(Step{ID: StepRunGuest, Run: func(ctx context.Context, st *RunState) error {
		st.Reply = "pong"
		return nil
	}})
	wf.Watch(StepRunGuest, After, func(ctx context.Context, ev Event) error {
		afterReply = ev.State.Reply
		return errors.New("ignored")
	})
	if err := (Runner{}).Run(context.Background(), wf, &RunState{}); err != nil {
		t.Fatal(err)
	}
	if afterReply != "pong" {
		t.Fatalf("reply=%q", afterReply)
	}
}

func TestBusGateOrderWithWatch(t *testing.T) {
	var log []string
	wf := New(Step{ID: "a", Run: func(ctx context.Context, st *RunState) error {
		log = append(log, "run")
		return nil
	}})
	wf.Watch("a", Before, func(ctx context.Context, ev Event) error {
		log = append(log, "watch-before")
		return nil
	})
	wf.On("a", Before, func(ctx context.Context, ev Event) error {
		log = append(log, "gate-before")
		return nil
	})
	wf.On("a", After, func(ctx context.Context, ev Event) error {
		log = append(log, "gate-after")
		return nil
	})
	wf.Watch("a", After, func(ctx context.Context, ev Event) error {
		log = append(log, "watch-after")
		return nil
	})
	if err := (Runner{}).Run(context.Background(), wf, &RunState{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"gate-before", "watch-before", "run", "gate-after", "watch-after"}
	if len(log) != len(want) {
		t.Fatalf("log=%v want %v", log, want)
	}
	for i := range want {
		if log[i] != want[i] {
			t.Fatalf("log=%v want %v", log, want)
		}
	}
}
