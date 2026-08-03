package toolhost

import (
	"strings"
	"testing"
)

func TestParseWriteFileJSON(t *testing.T) {
	path, content, err := ParseWriteFile(`{"rel_path":"user/a.md","content":"hello"}`)
	if err != nil {
		t.Fatal(err)
	}
	if path != "user/a.md" || content != "hello" {
		t.Fatalf("got %q %q", path, content)
	}
}

func TestParseWriteFilePlain(t *testing.T) {
	raw := "user/草案/第1章.md\n\n# 标题\n正文「引号」与 \"ascii\"\n第二行"
	path, content, err := ParseWriteFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if path != "user/草案/第1章.md" {
		t.Fatalf("path=%q", path)
	}
	if !strings.Contains(content, "正文「引号」") || !strings.Contains(content, "第二行") {
		t.Fatalf("content=%q", content)
	}
}

func TestParseWriteFilePlainWithKey(t *testing.T) {
	raw := "rel_path: notes/x.md\n\nbody"
	path, content, err := ParseWriteFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if path != "notes/x.md" || content != "body" {
		t.Fatalf("got %q %q", path, content)
	}
}

func TestParseWriteFileTruncatedJSON(t *testing.T) {
	// 模拟大 content 被截断：无闭合引号与 }
	raw := `{"rel_path":"user/ch.md","content":"# 章\n这是很长的正文，含「引号」与换行`
	path, content, err := ParseWriteFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if path != "user/ch.md" {
		t.Fatalf("path=%q", path)
	}
	if !strings.Contains(content, "很长的正文") || !strings.Contains(content, "「引号」") {
		t.Fatalf("content=%q", content)
	}
}

func TestParseWriteFileUnescapedNewlineRepair(t *testing.T) {
	// content 内字面换行（非法 JSON），修复后应可解析
	raw := "{\"rel_path\":\"a.md\",\"content\":\"line1\nline2\"}"
	path, content, err := ParseWriteFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if path != "a.md" || content != "line1\nline2" {
		t.Fatalf("got %q %q", path, content)
	}
}

func TestEncodeWriteFileJSON(t *testing.T) {
	s := EncodeWriteFileJSON("a.md", "x\"y")
	path, content, err := ParseWriteFile(s)
	if err != nil {
		t.Fatal(err)
	}
	if path != "a.md" || content != "x\"y" {
		t.Fatalf("got %q %q", path, content)
	}
}
