package toolgateway

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"ningharness/session"
	"ningharness/workspace"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestApplyToolOutputBoundArchives(t *testing.T) {
	root := t.TempDir()
	ws := workspace.New()
	if _, err := ws.Open(root); err != nil {
		t.Fatal(err)
	}
	h := New(ws, session.NewStore())
	h.SetTurnContext("task-trunc", "main", "")
	long := strings.Repeat("甲", maxToolVisibleRunes+100)
	res := &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(long)}}
	out := h.applyToolOutputBound("c1", "read_file", res)
	text := FormatToolResult(out)
	if utf8.RuneCountInString(text) <= maxToolVisibleRunes {
		t.Fatal("expected truncation note to grow past window slightly, or at least contain marker")
	}
	if !strings.Contains(text, "[truncated:") || !strings.Contains(text, ".ningharness/truncated/") {
		t.Fatalf("text=%s", text[:200])
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".ningharness", "truncated", "task-trunc", "*.txt"))
	if len(matches) != 1 {
		t.Fatalf("archive files=%v", matches)
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if utf8.RuneCountInString(string(raw)) < maxToolVisibleRunes {
		t.Fatal("archive should keep full text")
	}
	_ = context.Background()
}
