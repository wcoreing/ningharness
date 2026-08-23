package history

import (
	"strings"
	"testing"

	"github.com/wcoreing/ningharness/store"
)

func TestAppendLoadByTaskAndStripSnapshot(t *testing.T) {
	store.ResetCacheForTest()
	defer store.ResetCacheForTest()

	root := t.TempDir()
	if _, err := store.OpenProject(root); err != nil {
		t.Fatal(err)
	}

	sys := "you are desk"
	if err := EnsureSystem(root, "main", sys); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSystem(root, "main", sys+" v2"); err != nil {
		t.Fatal(err)
	}
	all, err := Load(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	sysCount := 0
	for _, m := range all {
		if m.Role == "system" {
			sysCount++
			if m.Content != sys+" v2" {
				t.Fatalf("system=%q", m.Content)
			}
		}
	}
	if sysCount != 1 {
		t.Fatalf("sysCount=%d want 1", sysCount)
	}

	run1 := "run-1"
	if err := Append(root, "main", Msg{
		Role: "user", Content: "写第一章", Feedforward: "## 项目现状\n- a", TaskID: run1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := Append(root, "main", Msg{
		Role: "assistant", TaskID: run1,
		ToolCallsJSON: EncodeToolCalls([]ToolCallSpec{{ID: "c1", Name: "write_file", Arguments: `{"rel_path":"a.md"}`}}),
	}); err != nil {
		t.Fatal(err)
	}
	if err := Append(root, "main", Msg{Role: "tool", ToolCallID: "c1", Content: "Successfully wrote a.md", TaskID: run1}); err != nil {
		t.Fatal(err)
	}
	if err := Append(root, "main", Msg{Role: "assistant", Content: "已落盘", TaskID: run1}); err != nil {
		t.Fatal(err)
	}

	byRun, err := LoadByTask(root, run1)
	if err != nil {
		t.Fatal(err)
	}
	if len(byRun) != 4 {
		t.Fatalf("byRun len=%d %+v", len(byRun), byRun)
	}
	if byRun[0].Feedforward == "" || byRun[0].Content != "写第一章" {
		t.Fatalf("user feedforward/content: %+v", byRun[0])
	}

	run2 := "run-2"
	if err := Append(root, "main", Msg{
		Role: "user", Content: "继续", Feedforward: "## 项目现状\n- b", TaskID: run2,
	}); err != nil {
		t.Fatal(err)
	}
	all, err = Load(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	built := BuildForModel(all, DefaultBudget(), true)
	var users []Msg
	for _, m := range built {
		if m.Role == "user" {
			users = append(users, m)
		}
		if m.Role == "system" {
			t.Fatal("skipLeadingSystem failed")
		}
	}
	if len(users) < 2 {
		t.Fatalf("users=%d", len(users))
	}
	// ContextMessages：只保留原话，不带旧轮前馈 / 内嵌快照
	if users[0].Content != "写第一章" || users[0].Feedforward != "" {
		t.Fatalf("old user context: %+v", users[0])
	}
	if users[len(users)-1].Content != "继续" || users[len(users)-1].Feedforward != "" {
		t.Fatalf("latest user context: %+v", users[len(users)-1])
	}
	wired := WireUser(Msg{Role: "user", Content: "继续", Feedforward: "## 项目现状\n- b"})
	if !strings.Contains(wired, SnapshotStartMarker) || !strings.Contains(wired, "继续") {
		t.Fatalf("WireUser=%q", wired)
	}
}

func TestIsUIRoleShowsAssistantWithToolCalls(t *testing.T) {
	if !IsUIRole(Msg{Role: "assistant", Content: "旁白", ToolCallsJSON: `[{"id":"a"}]`}) {
		t.Fatal("tool_calls assistant with content should show")
	}
	if !IsUIRole(Msg{Role: "assistant", Content: "", ToolCallsJSON: `[{"id":"a"}]`}) {
		t.Fatal("tools-only assistant should show (collapsed tools)")
	}
	if !IsUIRole(Msg{Role: "assistant", Content: "最终回复"}) {
		t.Fatal("final assistant should show")
	}
	if IsUIRole(Msg{Role: "assistant", Content: "", ToolCallsJSON: ""}) {
		t.Fatal("empty assistant should hide")
	}
}

func TestRepairChainDropsPartialToolsWhenStripping(t *testing.T) {
	in := []Msg{
		{Role: "user", Content: "删"},
		{
			Role: "assistant", Content: "开始删",
			ToolCallsJSON: EncodeToolCalls([]ToolCallSpec{
				{ID: "a", Name: "remove_container", Arguments: "{}"},
				{ID: "b", Name: "remove_container", Arguments: "{}"},
			}),
		},
		{Role: "tool", ToolCallID: "a", Content: "ok"},
		// 缺 b
	}
	out := RepairChain(in)
	for _, m := range out {
		if m.Role == "tool" {
			t.Fatalf("orphan tool kept after strip: %+v", out)
		}
		if m.Role == "assistant" && strings.Contains(m.ToolCallsJSON, `"a"`) {
			t.Fatalf("incomplete tool_calls kept: %+v", out)
		}
	}
	var sawAsst bool
	for _, m := range out {
		if m.Role == "assistant" && m.Content == "开始删" {
			sawAsst = true
		}
	}
	if !sawAsst {
		t.Fatalf("assistant content dropped: %+v", out)
	}
}

func TestRepairChainDropsOrphanToolCalls(t *testing.T) {
	in := []Msg{
		{Role: "user", Content: "写"},
		{Role: "assistant", ToolCallsJSON: EncodeToolCalls([]ToolCallSpec{{ID: "c1", Name: "write_file", Arguments: "{}"}})},
		// 缺 tool 结果
		{Role: "user", Content: "继续"},
		{Role: "assistant", ToolCallsJSON: EncodeToolCalls([]ToolCallSpec{{ID: "c2", Name: "write_file", Arguments: "{}"}})},
		{Role: "tool", ToolCallID: "c2", Content: "ok"},
		{Role: "assistant", Content: "好了"},
	}
	out := RepairChain(in)
	// 第一条残缺 assistant（仅 tool_calls）应被丢掉
	for _, m := range out {
		if m.Role == "assistant" && strings.Contains(m.ToolCallsJSON, `"c1"`) {
			t.Fatalf("orphan c1 kept: %+v", out)
		}
	}
	var sawC2 bool
	for i, m := range out {
		if m.Role == "assistant" && strings.Contains(m.ToolCallsJSON, `"c2"`) {
			if i+1 >= len(out) || out[i+1].Role != "tool" || out[i+1].ToolCallID != "c2" {
				t.Fatalf("c2 not followed by tool: %+v", out)
			}
			sawC2 = true
		}
	}
	if !sawC2 {
		t.Fatalf("missing intact c2 chain: %+v", out)
	}
}

func TestDefaultBudgetKeepsRecent(t *testing.T) {
	msgs := make([]Msg, 0, 80)
	for i := 0; i < 80; i++ {
		msgs = append(msgs, Msg{Role: "user", Content: "x"})
	}
	out := BuildForModel(msgs, DefaultBudget(), false)
	want := DefaultBudget().RecentKeep
	if len(out) != want {
		t.Fatalf("len=%d want %d", len(out), want)
	}
}

func TestBuildForModelSkipsThinking(t *testing.T) {
	in := []Msg{
		{Role: "user", Content: "写"},
		{Role: "thinking", Content: "内部推理"},
		{Role: "assistant", Content: "好了"},
	}
	out := BuildForModel(in, DefaultBudget(), false)
	for _, m := range out {
		if m.Role == "thinking" {
			t.Fatalf("thinking leaked: %+v", out)
		}
	}
	if len(out) != 2 {
		t.Fatalf("len=%d %+v", len(out), out)
	}
	ctxMsgs := ToContextMessages(in)
	for _, p := range ctxMsgs {
		if p.Role == "thinking" {
			t.Fatalf("ToContextMessages thinking: %+v", ctxMsgs)
		}
	}
}

func TestAppendThinkingMerges(t *testing.T) {
	store.ResetCacheForTest()
	defer store.ResetCacheForTest()
	root := t.TempDir()
	if _, err := store.OpenProject(root); err != nil {
		t.Fatal(err)
	}
	if err := AppendThinking(root, "main", "t1", "甲"); err != nil {
		t.Fatal(err)
	}
	if err := AppendThinking(root, "main", "t1", "乙"); err != nil {
		t.Fatal(err)
	}
	msgs, err := LoadByTask(root, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].Content != "甲乙" {
		t.Fatalf("%+v", msgs)
	}
}
