package workspace

import (
	"testing"

	"github.com/wcoreing/ningharness/protocol"
)

func TestCompactListingForAgent(t *testing.T) {
	kids := make([]protocol.TreeNode, 50)
	for i := range kids {
		kids[i] = protocol.TreeNode{RelPath: "古龙/a.txt", Name: "a.txt", WordCount: 10}
	}
	in := TreeListing{Nodes: []protocol.TreeNode{{
		RelPath: "古龙", Name: "古龙", IsDir: true, WordCount: 500, Children: kids,
	}}}
	out := CompactListingForAgent(in, 40)
	if !out.Truncated {
		t.Fatal("want truncated")
	}
	if len(out.Nodes) != 1 || len(out.Nodes[0].Children) != 41 {
		t.Fatalf("children=%d", len(out.Nodes[0].Children))
	}
	last := out.Nodes[0].Children[40]
	if last.Name == "" || last.RelPath != "古龙/…" {
		t.Fatalf("%+v", last)
	}
	if out.Nodes[0].WordCount != 500 {
		t.Fatalf("dir words should keep full sum, got %d", out.Nodes[0].WordCount)
	}
}
