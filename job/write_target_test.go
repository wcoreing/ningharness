package job

import (
	"strings"
	"testing"
)

func TestInjectWriteTargetPrefersTarget(t *testing.T) {
	feed := "## 提炼\n- **本轮只写**：`system/提炼/旧.md`（候选）\n- **模式**：单文件\n"
	got := InjectWriteTarget(feed, "system/提炼/新.md")
	if !strings.HasPrefix(got, "- **本轮只写**：`system/提炼/新.md`\n") {
		t.Fatalf("got=%q", got)
	}
	if strings.Contains(got, "旧.md") {
		t.Fatalf("old target should be stripped: %q", got)
	}
	if !strings.Contains(got, "## 提炼") || !strings.Contains(got, "单文件") {
		t.Fatalf("rest kept: %q", got)
	}
}

func TestInjectWriteTargetEmptyFeed(t *testing.T) {
	got := InjectWriteTarget("", "章节/开篇/01.md")
	if got != "- **本轮只写**：`章节/开篇/01.md`" {
		t.Fatalf("%q", got)
	}
}

func TestInjectWriteTargetNoRel(t *testing.T) {
	in := "- **模式**：多文件"
	if InjectWriteTarget(in, "") != in {
		t.Fatal("unchanged")
	}
}

func TestInjectWriteTargetStripsBlockList(t *testing.T) {
	feed := "## 当前宇宙\n- **本轮只写**：\n  - `system/训练/开篇/01.md`\n  - `system/训练/开篇/02.md`\n- **落点**：正稿只读\n"
	got := InjectWriteTarget(feed, "system/训练/开篇/03.md")
	if !strings.HasPrefix(got, "- **本轮只写**：`system/训练/开篇/03.md`\n") {
		t.Fatalf("got=%q", got)
	}
	if strings.Contains(got, "01.md") || strings.Contains(got, "02.md") {
		t.Fatalf("old block kept: %q", got)
	}
	if !strings.Contains(got, "当前宇宙") || !strings.Contains(got, "落点") {
		t.Fatalf("rest kept: %q", got)
	}
}
