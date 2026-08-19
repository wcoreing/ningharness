package workspace

import (
	"os"
	"testing"
)

func TestDecodeTextBytesUTF8(t *testing.T) {
	got := DecodeTextBytes([]byte("你好世界\n"))
	if got != "你好世界\n" {
		t.Fatalf("got %q", got)
	}
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hi")...)
	if DecodeTextBytes(bom) != "hi" {
		t.Fatal("utf8 bom")
	}
}

func TestDecodeTextBytesGBK(t *testing.T) {
	// 「大旗英雄传」GBK
	gbk := []byte{0xB4, 0xF3, 0xC6, 0xEC, 0xD3, 0xA2, 0xD0, 0xDB, 0xB4, 0xAB}
	got := DecodeTextBytes(gbk)
	if got != "大旗英雄传" {
		t.Fatalf("gbk got %q", got)
	}
	if !NeedsUTF8Rewrite(gbk) {
		t.Fatal("gbk should need rewrite")
	}
	if NeedsUTF8Rewrite([]byte("utf8 ok")) {
		t.Fatal("utf8 should not need rewrite")
	}
}

func TestReadTextDecodesGBK(t *testing.T) {
	root := t.TempDir()
	gbk := []byte{0xB4, 0xF3, 0xC6, 0xEC, 0xD3, 0xA2, 0xD0, 0xDB, 0xB4, 0xAB, '\n'}
	path := root + "/novel.txt"
	if err := os.WriteFile(path, gbk, 0o644); err != nil {
		t.Fatal(err)
	}
	s := New()
	if _, err := s.Open(root); err != nil {
		t.Fatal(err)
	}
	body, err := s.ReadText("novel.txt")
	if err != nil {
		t.Fatal(err)
	}
	if body != "大旗英雄传\n" {
		t.Fatalf("got %q", body)
	}
}
