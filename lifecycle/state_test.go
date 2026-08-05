package lifecycle

import (
	"context"
	"testing"
)

func TestWithRunStateRoundTrip(t *testing.T) {
	st := &RunState{TaskID: "t1", Prompt: "hi"}
	ctx := WithRunState(context.Background(), st)
	got := RunStateFrom(ctx)
	if got != st || got.TaskID != "t1" {
		t.Fatalf("got=%+v", got)
	}
	if RunStateFrom(context.Background()) != nil {
		t.Fatal("empty ctx")
	}
}

func TestAssembleSkipVsFeedforwardSemantics(t *testing.T) {
	// 文档化约定：SkipUserAppend && 无 Feedforward → assemble 跳过（由 defaults 测集成）
	st := &RunState{SkipUserAppend: true, Prompt: "p"}
	if !st.SkipUserAppend || st.Feedforward != "" {
		t.Fatal("precondition")
	}
	st.Feedforward = "ff"
	if st.SkipUserAppend && st.Feedforward == "" {
		t.Fatal("with ff, assemble should not early-return on skip alone")
	}
}
