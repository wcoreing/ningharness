package task

import (
	"strings"
	"testing"
)

func TestFormatToolDigestLines(t *testing.T) {
	if got := FormatToolDigestLines(nil); got != nil {
		t.Fatal(got)
	}
	got := FormatToolDigestLines([]ToolCall{
		{Name: "write_file", OK: true, Path: "a.md"},
		{Name: "read_file", OK: false},
	})
	if len(got) != 2 || got[0] != "write_file→a.md" || got[1] != "read_file(fail)" {
		t.Fatal(got)
	}
}

func TestCleanAssistantTextStripsWriteGateNote(t *testing.T) {
	in := "已落盘：a.md\n\n（系统：本轮未见写盘成功回执，上文未落盘。）已落盘：a.md（10 字）"
	got := CleanAssistantText(in)
	if strings.Contains(got, "未见写盘") {
		t.Fatalf("note remains: %q", got)
	}
	if !strings.Contains(got, "已落盘") {
		t.Fatalf("body lost: %q", got)
	}
}
