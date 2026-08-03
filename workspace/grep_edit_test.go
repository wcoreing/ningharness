package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepLiteralAndGlob(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("章节/一.md", "燕追风拔剑。\n旁人后退。\n")
	mustWrite("章节/二.md", "雨夜。\n燕追风再起。\n")
	mustWrite("设定/刀.md", "燕追风的刀。\n")
	mustWrite("readme.txt", "no match here\n")

	s := New()
	if _, err := s.Open(root); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Grep(GrepOpts{Pattern: "燕追风", Glob: "*.md", MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 3 {
		t.Fatalf("hits=%d %#v", len(hits), hits)
	}
	hits, err = s.Grep(GrepOpts{Pattern: "燕追风", Path: "设定", MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].RelPath != "设定/刀.md" {
		t.Fatalf("scoped %#v", hits)
	}
	hits, err = s.Grep(GrepOpts{Pattern: "燕.", Regex: true, Path: "章节/一.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Text, "燕追风") {
		t.Fatalf("regex %#v", hits)
	}
}

func TestApplyEdit(t *testing.T) {
	body := "aaa\nbbb\naaa\n"
	_, n, err := ApplyEdit(body, "aaa", "xxx", false)
	if err == nil || n != 2 {
		t.Fatalf("want multi err n=2 got n=%d err=%v", n, err)
	}
	out, n, err := ApplyEdit(body, "bbb", "ccc", false)
	if err != nil || n != 1 || !strings.Contains(out, "ccc") {
		t.Fatalf("single got n=%d out=%q err=%v", n, out, err)
	}
	out, n, err = ApplyEdit(body, "aaa", "z", true)
	if err != nil || n != 2 || strings.Count(out, "z") != 2 {
		t.Fatalf("replaceAll n=%d out=%q err=%v", n, out, err)
	}
}
