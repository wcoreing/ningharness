package task

import (
	"strings"
	"testing"
)

func TestFormatSummaryShortNoDump(t *testing.T) {
	rec := &Record{
		ID:          "task-1",
		Driver:      "wnai",
		Status:      "ok",
		Feedforward: "## 项目现状\n- a",
		Tools: []ToolCall{
			{Name: "read_file", OK: true, Path: "a.md", Detail: "resource#9 · read_file · result · ok · a.md · 9000字 · 全文 desk_session recall_resource resource_id=9"},
		},
		Steps: []Step{
			{Kind: "thinking", Text: "先读文件"},
			{Kind: "tool", Name: "read_file", Args: "resource#8", Result: "resource#9", OK: true, Done: true},
		},
	}
	got := FormatSummary(rec)
	if !strings.Contains(got, "task `task-1`") {
		t.Fatalf("%s", got)
	}
	if !strings.Contains(got, "feedforward:") {
		t.Fatalf("want feedforward: %s", got)
	}
	if !strings.Contains(got, "#8") || !strings.Contains(got, "#9") {
		t.Fatalf("want resource ids: %s", got)
	}
	if !strings.Contains(got, "recall_resource") {
		t.Fatalf("%s", got)
	}
	if strings.Contains(got, "9000字 · 全文") {
		t.Fatalf("should strip long hint noise: %s", got)
	}
	if strings.Contains(got, `"steps"`) || strings.Contains(got, `"tools"`) {
		t.Fatalf("must not dump json: %s", got)
	}
}
