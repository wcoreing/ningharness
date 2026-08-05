package goal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteOnceAndReadStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "GOAL.yaml")
	if err := WriteOnce(path, "make tests pass"); err != nil {
		t.Fatal(err)
	}
	if got := ReadStatus(path); got != StatusActive {
		t.Fatalf("status=%q", got)
	}
	if err := os.WriteFile(path, []byte("objective: x\nstatus: complete\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadStatus(path); got != StatusComplete {
		t.Fatalf("status=%q", got)
	}
}

func TestReadStatusBadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.yaml")
	if got := ReadStatus(path); got != StatusBlocked {
		t.Fatalf("missing -> %q", got)
	}
	path = filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte(":::not yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadStatus(path); got != StatusBlocked {
		t.Fatalf("bad yaml -> %q", got)
	}
	if err := os.WriteFile(path, []byte("objective: x\nstatus: weird\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadStatus(path); got != StatusBlocked {
		t.Fatalf("weird -> %q", got)
	}
}

func TestRunCompletesOnThirdRound(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, "GOAL.yaml")
	var rounds []int
	out, err := Run(context.Background(), Spec{
		Objective:   "ship it",
		ControlPath: control,
		MaxRounds:   10,
	}, func(ctx context.Context, wire string, round int) error {
		rounds = append(rounds, round)
		if !strings.Contains(wire, "[goal]") || !strings.Contains(wire, "ship it") {
			t.Fatalf("wire missing goal block: %q", wire)
		}
		if round < 3 {
			return nil
		}
		return os.WriteFile(control, []byte("objective: ship it\nstatus: complete\n"), 0o644)
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != OutcomeComplete {
		t.Fatalf("outcome=%q", out)
	}
	if len(rounds) != 3 {
		t.Fatalf("rounds=%v", rounds)
	}
}

func TestRunBlocked(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, "GOAL.yaml")
	out, err := Run(context.Background(), Spec{
		Objective:   "need help",
		ControlPath: control,
		MaxRounds:   5,
	}, func(ctx context.Context, wire string, round int) error {
		return os.WriteFile(control, []byte("objective: need help\nstatus: blocked\n"), 0o644)
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != OutcomeBlocked {
		t.Fatalf("outcome=%q", out)
	}
}

func TestRunMaxRounds(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, "GOAL.yaml")
	out, err := Run(context.Background(), Spec{
		Objective:   "never ends",
		ControlPath: control,
		MaxRounds:   2,
	}, func(ctx context.Context, wire string, round int) error {
		return nil
	}, nil)
	if err == nil {
		t.Fatal("want max rounds error")
	}
	if out != OutcomeMaxRounds {
		t.Fatalf("outcome=%q", out)
	}
}

func TestRunCancel(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, "GOAL.yaml")
	ctx, cancel := context.WithCancel(context.Background())
	out, err := Run(ctx, Spec{
		Objective:   "abort me",
		ControlPath: control,
		MaxRounds:   10,
	}, func(ctx context.Context, wire string, round int) error {
		cancel()
		return ctx.Err()
	}, nil)
	if out != OutcomeAborted {
		t.Fatalf("outcome=%q err=%v", out, err)
	}
}
