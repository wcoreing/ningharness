package workspace

import (
	"strings"
	"testing"
)

func TestFormatEditOK(t *testing.T) {
	got := FormatEditOK("a.md", 1, "甲乙", "mcp-edit-1")
	if !strings.Contains(got, "Successfully edited 'a.md'（1 处 · 2 字）") {
		t.Fatalf("got=%s", got)
	}
	if !strings.Contains(got, "writeId=mcp-edit-1") {
		t.Fatalf("writeId: %s", got)
	}
	got = FormatEditOK("a.md", 2, "x", "")
	if !strings.Contains(got, "2 处") || strings.Contains(got, "writeId=") {
		t.Fatalf("got=%s", got)
	}
}
