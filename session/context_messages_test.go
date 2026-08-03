package session

import (
	"strings"
	"testing"
)

func TestBuildContextMessagesRecentOnly(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
	}
	out := BuildContextMessages(msgs, DefaultContextBudget(), nil)
	if len(out) != 2 {
		t.Fatalf("%#v", out)
	}
	if out[0].Content != "a" || out[1].Content != "b" {
		t.Fatalf("%#v", out)
	}
}

func TestBuildContextMessagesFoldsOlder(t *testing.T) {
	var msgs []Message
	for i := 0; i < 20; i++ {
		msgs = append(msgs, Message{Role: "user", Content: "u" + string(rune('0'+i%10))})
		msgs = append(msgs, Message{Role: "assistant", Content: "a" + string(rune('0'+i%10))})
	}
	b := DefaultContextBudget()
	b.RecentKeep = 4
	b.OlderUserKeep = 3
	out := BuildContextMessages(msgs, b, nil)
	if len(out) != 5 { // 1 summary + 4 recent
		t.Fatalf("len=%d %#v", len(out), out)
	}
	if out[0].Role != "system" || !strings.Contains(out[0].Content, "已折叠") {
		t.Fatalf("want summary: %#v", out[0])
	}
	if !strings.Contains(out[0].Content, "search_session") {
		t.Fatalf("want search_session hint: %#v", out[0])
	}
}

func TestBuildContextMessagesToolsViaLookupNotAssistantMouth(t *testing.T) {
	lookup := func(taskID string) []string {
		if taskID == "run-1" {
			return []string{"write_file→a.md"}
		}
		return nil
	}
	var msgs []Message
	for i := 0; i < 10; i++ {
		msgs = append(msgs, Message{Role: "user", Content: "q" + string(rune('0'+i))})
		msgs = append(msgs, Message{Role: "assistant", Content: "写完了", TaskID: "run-1"})
	}
	b := DefaultContextBudget()
	b.RecentKeep = 2
	b.OlderToolKeep = 3
	out := BuildContextMessages(msgs, b, lookup)
	if len(out) < 1 || out[0].Role != "system" {
		t.Fatalf("%#v", out)
	}
	if !strings.Contains(out[0].Content, "write_file→a.md") {
		t.Fatalf("want digest in fold summary: %s", out[0].Content)
	}
	for _, p := range out {
		if p.Role == "assistant" && strings.Contains(p.Content, "[Desk tools:") {
			t.Fatalf("assistant prior must not contain Desk tools: %#v", p)
		}
		if p.Role == "assistant" && strings.Contains(p.Content, "write_file→") {
			t.Fatalf("assistant prior must not embed digest: %#v", p)
		}
	}
}

func TestBuildContextMessagesStripsLegacyFootprintFromAssistant(t *testing.T) {
	out := BuildContextMessages([]Message{
		{Role: "assistant", Content: "[Desk tools: none]\n\n你好\n\n[Desk tools: write_file]"},
	}, DefaultContextBudget(), nil)
	if len(out) != 1 || out[0].Content != "你好" {
		t.Fatalf("%#v", out)
	}
}

func TestBuildContextMessagesTruncatesLong(t *testing.T) {
	long := strings.Repeat("字", 100)
	out := BuildContextMessages([]Message{{Role: "user", Content: long}}, ContextBudget{
		RecentKeep:  2,
		MaxMsgRunes: 10,
	}, nil)
	if len(out) != 1 {
		t.Fatal(out)
	}
	r := []rune(out[0].Content)
	if len(r) > 11 {
		t.Fatalf("%d %q", len(r), out[0].Content)
	}
}

func TestRewindKeepsPrefix(t *testing.T) {
	root := t.TempDir()
	pid := "p"
	st := NewStore()
	_, _ = st.Snapshot(root, pid)
	_, _ = st.Append(root, pid, "main", "user", "1", "", "")
	_, _ = st.Append(root, pid, "main", "assistant", "2", "run-1", "")
	_, _ = st.Append(root, pid, "main", "user", "3", "", "")
	f, err := st.Rewind(root, pid, "main", 2)
	if err != nil {
		t.Fatal(err)
	}
	var cur *Session
	for i := range f.Sessions {
		if f.Sessions[i].ID == "main" {
			cur = &f.Sessions[i]
			break
		}
	}
	if cur == nil || len(cur.Messages) != 2 {
		t.Fatalf("%#v", cur)
	}
	if cur.Messages[0].Content != "1" || cur.Messages[1].Content != "2" {
		t.Fatalf("%#v", cur.Messages)
	}
}
