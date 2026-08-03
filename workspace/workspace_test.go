package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ningharness/protocol"
)

func TestOpenListReadWrite(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.md"), []byte("# hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New()
	if _, err := s.Open(root); err != nil {
		t.Fatal(err)
	}
	listing, err := s.ListTree()
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Nodes) == 0 {
		t.Fatal("expected non-empty tree")
	}
	var hello *protocol.TreeNode
	for i := range listing.Nodes {
		if listing.Nodes[i].Name == "hello.md" {
			hello = &listing.Nodes[i]
			break
		}
	}
	if hello == nil || hello.WordCount < 1 {
		t.Fatalf("expected hello.md wordCount > 0, got %#v", hello)
	}
	body, err := s.ReadText("hello.md")
	if err != nil || body != "# hi\n" {
		t.Fatalf("read: %q %v", body, err)
	}
	if err := s.WriteText("notes/a.md", "x", "test"); err != nil {
		t.Fatal(err)
	}
	body, err = s.ReadText("notes/a.md")
	if err != nil || body != "x" {
		t.Fatalf("nested read: %q %v", body, err)
	}
	if _, err := s.Resolve("../outside"); err == nil {
		t.Fatal("expected escape reject")
	}
}

func TestNonTextReject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.png"), []byte{0x89, 0x50}, 0o644); err != nil {
		t.Fatal(err)
	}
	s := New()
	if _, err := s.Open(root); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadText("a.png"); err == nil {
		t.Fatal("expected non-text reject")
	}
}

func TestReadDataURLSniffsJPEGMisnamedPNG(t *testing.T) {
	root := t.TempDir()
	// JFIF 最小头：FF D8 FF + 若干填充
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	if err := os.WriteFile(filepath.Join(root, "scene.png"), jpeg, 0o644); err != nil {
		t.Fatal(err)
	}
	s := New()
	if _, err := s.Open(root); err != nil {
		t.Fatal(err)
	}
	url, err := s.ReadDataURL("scene.png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "data:image/jpeg;base64,") {
		t.Fatalf("want jpeg data URL, got %q", url[:min(48, len(url))])
	}
}

func TestConcurrentOpenList(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.md"), []byte("a"), 0o644)
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Open(root)
			_, _ = s.ListTree()
			_, _ = s.ReadText("a.md")
		}()
	}
	wg.Wait()
	if s.Current() == nil {
		t.Fatal("expected project")
	}
}

func TestTreeListsDeep(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "d1", "d2", "d3", "d4")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(deep, "x.md"), []byte("x"), 0o644)
	s := New()
	if _, err := s.Open(root); err != nil {
		t.Fatal(err)
	}
	listing, err := s.ListTree()
	if err != nil {
		t.Fatal(err)
	}
	if listing.Truncated {
		t.Fatal("truncation removed; expected Truncated=false")
	}
	// d1 → d2 → d3 → d4 → x.md
	if len(listing.Nodes) != 1 || listing.Nodes[0].Name != "d1" {
		t.Fatalf("root child: %+v", listing.Nodes)
	}
	cur := listing.Nodes[0]
	for _, want := range []string{"d2", "d3", "d4"} {
		if len(cur.Children) != 1 || cur.Children[0].Name != want {
			t.Fatalf("want %s under %s, got %+v", want, cur.Name, cur.Children)
		}
		cur = cur.Children[0]
	}
	if len(cur.Children) != 1 || cur.Children[0].Name != "x.md" {
		t.Fatalf("want x.md, got %+v", cur.Children)
	}
}
