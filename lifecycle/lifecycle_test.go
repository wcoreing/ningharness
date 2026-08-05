package lifecycle

import (
	"context"
	"errors"
	"testing"
)

func TestRunnerOrderAndAfterSkipOnRunError(t *testing.T) {
	var log []string
	wf := New(
		Step{ID: "a", Run: func(ctx context.Context, st *RunState) error {
			log = append(log, "run:a")
			return nil
		}},
		Step{ID: "b", Run: func(ctx context.Context, st *RunState) error {
			log = append(log, "run:b")
			return errors.New("boom")
		}},
		Step{ID: "c", Run: func(ctx context.Context, st *RunState) error {
			log = append(log, "run:c")
			return nil
		}},
	)
	wf.WithBefore("a", func(ctx context.Context, st *RunState) error {
		log = append(log, "before:a")
		return nil
	})
	wf.WithAfter("a", func(ctx context.Context, st *RunState) error {
		log = append(log, "after:a")
		return nil
	})
	wf.WithAfter("b", func(ctx context.Context, st *RunState) error {
		log = append(log, "after:b")
		return nil
	})

	st := &RunState{}
	err := (Runner{}).Run(context.Background(), wf, st)
	if err == nil {
		t.Fatal("expected error")
	}
	want := []string{"before:a", "run:a", "after:a", "run:b"}
	if len(log) != len(want) {
		t.Fatalf("log=%v want %v", log, want)
	}
	for i := range want {
		if log[i] != want[i] {
			t.Fatalf("log=%v want %v", log, want)
		}
	}
}

func TestBeforeBlockSkipsRun(t *testing.T) {
	ran := false
	wf := New(Step{ID: "a", Run: func(ctx context.Context, st *RunState) error {
		ran = true
		return nil
	}})
	wf.WithBefore("a", func(ctx context.Context, st *RunState) error {
		return ErrBlock
	})
	err := (Runner{}).Run(context.Background(), wf, &RunState{})
	if !errors.Is(err, ErrBlock) {
		t.Fatalf("err=%v", err)
	}
	if ran {
		t.Fatal("run should not execute")
	}
}

func TestOnExitAlwaysRuns(t *testing.T) {
	var exits []string
	wf := New(Step{ID: "a", Run: func(ctx context.Context, st *RunState) error {
		st.OnExit(func(err error) {
			if err != nil {
				exits = append(exits, "err")
			} else {
				exits = append(exits, "ok")
			}
		})
		return errors.New("fail")
	}})
	_ = (Runner{}).Run(context.Background(), wf, &RunState{})
	if len(exits) != 1 || exits[0] != "err" {
		t.Fatalf("exits=%v", exits)
	}

	exits = nil
	wf2 := New(Step{ID: "a", Run: func(ctx context.Context, st *RunState) error {
		st.OnExit(func(err error) {
			exits = append(exits, "ok")
		})
		return nil
	}})
	if err := (Runner{}).Run(context.Background(), wf2, &RunState{}); err != nil {
		t.Fatal(err)
	}
	if len(exits) != 1 || exits[0] != "ok" {
		t.Fatalf("exits=%v", exits)
	}
}

func TestInsertAfterAndClone(t *testing.T) {
	wf := New(
		Step{ID: StepAssembleContext, Run: func(ctx context.Context, st *RunState) error { return nil }},
		Step{ID: StepRunGuest, Run: func(ctx context.Context, st *RunState) error { return nil }},
	)
	if err := wf.InsertAfter(StepAssembleContext, Step{ID: "compact", Run: func(ctx context.Context, st *RunState) error {
		st.Reply = "x"
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	ids := []StepID{}
	for _, s := range wf.Steps() {
		ids = append(ids, s.ID)
	}
	if len(ids) != 3 || ids[1] != "compact" {
		t.Fatalf("ids=%v", ids)
	}
	cl := wf.Clone()
	cl.On(StepRunGuest, Before, func(ctx context.Context, ev Event) error { return ErrBlock })
	// 原生命周期未挂 Gate，仍应跑通
	if err := (Runner{}).Run(context.Background(), wf, &RunState{}); err != nil {
		t.Fatalf("original: %v", err)
	}
	err := (Runner{}).Run(context.Background(), cl, &RunState{})
	if !errors.Is(err, ErrBlock) {
		t.Fatalf("clone err=%v", err)
	}
}

type fakeHost struct {
	log *[]string
}

func (h fakeHost) BeginTask(ctx context.Context, st *RunState) error {
	*h.log = append(*h.log, "begin")
	return nil
}
func (h fakeHost) AssembleContext(ctx context.Context, st *RunState) error {
	*h.log = append(*h.log, "assemble")
	return nil
}
func (h fakeHost) RunGuest(ctx context.Context, st *RunState) error {
	*h.log = append(*h.log, "guest")
	st.Reply = "hi"
	return nil
}
func (h fakeHost) PersistTurn(ctx context.Context, st *RunState) error {
	*h.log = append(*h.log, "persist")
	return nil
}
func (h fakeHost) EndTask(ctx context.Context, st *RunState) error {
	*h.log = append(*h.log, "end")
	return nil
}

func TestNewDefaultOrder(t *testing.T) {
	var log []string
	wf := NewDefault(fakeHost{log: &log})
	st := &RunState{Prompt: "p"}
	if err := (Runner{}).Run(context.Background(), wf, st); err != nil {
		t.Fatal(err)
	}
	want := []string{"begin", "assemble", "guest", "persist", "end"}
	if len(log) != len(want) {
		t.Fatalf("log=%v", log)
	}
	for i := range want {
		if log[i] != want[i] {
			t.Fatalf("log=%v want %v", log, want)
		}
	}
	if st.Reply != "hi" {
		t.Fatalf("reply=%q", st.Reply)
	}
}
