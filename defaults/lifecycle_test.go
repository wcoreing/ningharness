package defaults

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ningharness"
	"ningharness/guest"
	"ningharness/lifecycle"
	"ningharness/memory"
)

func (g *stubGuest) Run(ctx context.Context, in guest.Input) (string, error) {
	g.calls++
	g.last = in
	return g.reply, g.err
}

type stubGuest struct {
	reply string
	err   error
	calls int
	last  guest.Input
}

func TestChatRunsDefaultLifecycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	rt, err := Open(Opts{
		Opts: ningharness.Opts{
			DataDir: filepath.Join(dir, "data"),
			Root:    root,
		},
		MCPAddr:     "off",
		WithoutEino: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if rt.Lifecycle == nil {
		t.Fatal("Lifecycle required")
	}
	g := &stubGuest{reply: "pong"}
	rt.SetGuest(g)
	reply, err := rt.Chat(t.Context(), "ping")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "pong" || g.calls != 1 {
		t.Fatalf("reply=%q calls=%d", reply, g.calls)
	}
}

func TestLifecycleGateBlockSkipsGuest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	rt, err := Open(Opts{
		Opts: ningharness.Opts{
			DataDir: filepath.Join(dir, "data"),
			Root:    root,
		},
		MCPAddr:     "off",
		WithoutEino: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	g := &stubGuest{reply: "should-not"}
	rt.SetGuest(g)
	lc := rt.Lifecycle.Clone()
	lc.On(lifecycle.StepRunGuest, lifecycle.Before, func(ctx context.Context, ev lifecycle.Event) error {
		return lifecycle.ErrBlock
	})
	rt.SetLifecycle(lc)
	_, err = rt.Chat(t.Context(), "hi")
	if !errors.Is(err, lifecycle.ErrBlock) {
		t.Fatalf("err=%v", err)
	}
	if g.calls != 0 {
		t.Fatalf("guest calls=%d", g.calls)
	}
}

func TestAssembleContextWritesFeedforward(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	rt, err := Open(Opts{
		Opts: ningharness.Opts{
			DataDir: filepath.Join(dir, "data"),
			Root:    root,
		},
		MCPAddr:     "off",
		WithoutEino: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	st := &lifecycle.RunState{
		Root:        root,
		SessionKey:  "main",
		TaskID:      "t-ff",
		Prompt:      "写一章",
		Feedforward: "## 现状\n- a",
	}
	if err := rt.AssembleContext(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	f, err := rt.Session.Snapshot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range f.Sessions {
		if s.ID != "main" {
			continue
		}
		for _, m := range s.Messages {
			if m.Role == "user" && m.Content == "写一章" && m.HasFeedforward {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected user with feedforward, sessions=%+v", f.Sessions)
	}
}

func TestAssembleContextMergesMemoryPatch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	rt, err := Open(Opts{
		Opts: ningharness.Opts{
			DataDir: filepath.Join(dir, "data"),
			Root:    root,
		},
		MCPAddr:     "off",
		WithoutEino: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	rt.SetMemory(stubMemory{patch: "## mem\n- x"})
	st := &lifecycle.RunState{
		Root:        root,
		SessionKey:  "main",
		TaskID:      "t-mem",
		Prompt:      "hi",
		Feedforward: "## client\n- y",
	}
	if err := rt.AssembleContext(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(st.Feedforward, "## client") || !strings.Contains(st.Feedforward, "## mem") {
		t.Fatalf("Feedforward=%q", st.Feedforward)
	}
}

func TestRunGuestPassesFeedforward(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	rt, err := Open(Opts{
		Opts: ningharness.Opts{
			DataDir: filepath.Join(dir, "data"),
			Root:    root,
		},
		MCPAddr:         "off",
		WithoutEino:     true,
		WithoutMemory:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	g := &stubGuest{reply: "pong"}
	rt.SetGuest(g)
	st := &lifecycle.RunState{
		Root:        root,
		SessionKey:  "main",
		TaskID:      "t-ff-run",
		Prompt:      "hi",
		Feedforward: "## ff",
	}
	if err := rt.RunGuest(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	if g.last.Message != "hi" || g.last.Feedforward != "## ff" {
		t.Fatalf("last=%+v", g.last)
	}
}

type stubMemory struct{ patch string }

func (s stubMemory) Assemble(ctx context.Context, in memory.AssembleInput) (string, error) {
	return s.patch, nil
}

func TestLifecycleWatchAfterGuest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	rt, err := Open(Opts{
		Opts: ningharness.Opts{
			DataDir: filepath.Join(dir, "data"),
			Root:    root,
		},
		MCPAddr:     "off",
		WithoutEino: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	rt.SetGuest(&stubGuest{reply: "pong"})
	var seen string
	lc := rt.Lifecycle.Clone()
	lc.Watch(lifecycle.StepRunGuest, lifecycle.After, func(ctx context.Context, ev lifecycle.Event) error {
		seen = ev.State.Reply
		return errors.New("watch must not fail chat")
	})
	rt.SetLifecycle(lc)
	reply, err := rt.Chat(t.Context(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "pong" || seen != "pong" {
		t.Fatalf("reply=%q seen=%q", reply, seen)
	}
}
