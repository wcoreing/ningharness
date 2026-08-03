package task

import (
	"strings"
	"testing"

	"ningharness/history"
)

func TestStepsFromHistoryCallResult(t *testing.T) {
	msgs := []history.Msg{
		{Role: "thinking", Content: "想一下"},
		{Role: "thinking", Content: "再想"},
		{Role: "assistant", ToolCallsJSON: history.EncodeToolCalls([]history.ToolCallSpec{
			{ID: "c1", Name: "read_file", Arguments: "resource#1"},
		})},
		{Role: "tool", ToolCallID: "c1", Content: "resource#2 · ok"},
	}
	steps := StepsFromHistory(msgs)
	if len(steps) != 2 {
		t.Fatalf("len=%d %+v", len(steps), steps)
	}
	if steps[0].Kind != "thinking" || steps[0].Text != "想一下再想" {
		t.Fatalf("thinking=%+v", steps[0])
	}
	if steps[1].Kind != "tool" || steps[1].Name != "read_file" || !steps[1].Done || steps[1].Args != "resource#1" {
		t.Fatalf("tool=%+v", steps[1])
	}
}

func TestStepsFromHistoryParallelSameName(t *testing.T) {
	msgs := []history.Msg{
		{Role: "assistant", ToolCallsJSON: history.EncodeToolCalls([]history.ToolCallSpec{
			{ID: "a", Name: "read_file", Arguments: "1"},
			{ID: "b", Name: "read_file", Arguments: "2"},
		})},
		{Role: "tool", ToolCallID: "b", Content: "rb"},
		{Role: "tool", ToolCallID: "a", Content: "ra"},
	}
	steps := StepsFromHistory(msgs)
	if len(steps) != 2 {
		t.Fatalf("%+v", steps)
	}
	if steps[0].CallID != "a" || steps[0].Result != "ra" || !steps[0].Done {
		t.Fatalf("a=%+v", steps[0])
	}
	if steps[1].CallID != "b" || steps[1].Result != "rb" {
		t.Fatalf("b=%+v", steps[1])
	}
}

func TestStepsFromHistoryResourceIDs(t *testing.T) {
	msgs := []history.Msg{
		{Role: "assistant", ToolCallsJSON: history.EncodeToolCalls([]history.ToolCallSpec{
			{ID: "c1", Name: "get_skill", Arguments: `{"skill":"extract"}`, ResourceIDs: []int64{10}},
		})},
		{Role: "tool", ToolCallID: "c1", Content: "ok", ResourceIDsJSON: `[11,12]`},
	}
	steps := StepsFromHistory(msgs)
	if len(steps) != 1 {
		t.Fatalf("%+v", steps)
	}
	got := steps[0].ResourceIDs
	if len(got) != 3 || got[0] != 10 || got[1] != 11 || got[2] != 12 {
		t.Fatalf("resourceIds=%v", got)
	}
}

func TestStepsFromHistorySkipUserAssistantText(t *testing.T) {
	steps := StepsFromHistory([]history.Msg{
		{Role: "user", Content: "写"},
		{Role: "assistant", Content: "好了"},
	})
	if len(steps) != 0 {
		t.Fatalf("%+v", steps)
	}
	_ = strings.TrimSpace
}
