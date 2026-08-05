package contextpatch

import (
	"strings"
	"testing"
)

func TestPinAddedTruncatesLongNote(t *testing.T) {
	long := strings.Repeat("尺", 300)
	p := PinAdded("user/_meta/写作提示词.md", long, 3)
	out := p.Format()
	if !strings.Contains(out, "ContextPatch") || !strings.Contains(out, "pin.added") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "how=`list_pins`") {
		t.Fatal(out)
	}
	if !strings.Contains(out, "说明已截断") {
		t.Fatal(out)
	}
	if strings.Count(out, "尺") > maxNoteDeltaRunes+5 {
		t.Fatalf("note not truncated enough")
	}
}

func TestAppendPreservesBase(t *testing.T) {
	got := Append("pinned x", PinAdded("a.md", "", 1))
	if !strings.HasPrefix(got, "pinned x") || !strings.Contains(got, "## ContextPatch") {
		t.Fatal(got)
	}
}

func TestFileWroteRefsReadFile(t *testing.T) {
	p := FileWrote("章节/一.md", 12, "w1")
	if p.Kind != KindFileWrote || len(p.Refs) != 1 || p.Refs[0].How != HowReadFile {
		t.Fatalf("%+v", p)
	}
}
