package history

import "testing"

func TestContextWindowMatchesBuildForModel(t *testing.T) {
	msgs := make([]Msg, 0, 55)
	for i := 1; i <= 55; i++ {
		msgs = append(msgs, Msg{Seq: i, Role: "user", Content: "hi"})
	}
	win := ContextWindow(msgs, DefaultBudget())
	in := BuildForModel(msgs, DefaultBudget(), true)
	if len(in) != DefaultBudget().RecentKeep {
		t.Fatalf("BuildForModel len=%d want %d", len(in), DefaultBudget().RecentKeep)
	}
	for _, m := range in {
		if !win.InWindowSeq[m.Seq] {
			t.Fatalf("seq %d in model but not in window", m.Seq)
		}
	}
	// 最早 7 条 UI 消息应不进模
	for seq := 1; seq <= 7; seq++ {
		if win.InWindowSeq[seq] {
			t.Fatalf("seq %d should be out of window", seq)
		}
	}
}

func TestContextWindowToolChainNotSplit(t *testing.T) {
	msgs := []Msg{
		{Seq: 1, Role: "user", Content: "go"},
		{Seq: 2, Role: "assistant", Content: "", ToolCallsJSON: `[{"id":"c1","name":"read_file","arguments":"{}"}]`},
		{Seq: 3, Role: "tool", Content: "ok", ToolCallID: "c1"},
	}
	// 填满使 tool 行靠近截断边界
	for i := 4; i <= 50; i++ {
		msgs = append(msgs, Msg{Seq: i, Role: "user", Content: "x"})
	}
	win := ContextWindow(msgs, DefaultBudget())
	in := BuildForModel(msgs, DefaultBudget(), true)
	hasTool := false
	for _, m := range in {
		if m.Role == "tool" {
			hasTool = true
		}
	}
	if hasTool && !win.InWindowSeq[3] {
		t.Fatal("tool row should stay in window when chain kept")
	}
}
