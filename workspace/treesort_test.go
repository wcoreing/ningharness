package workspace

import (
	"testing"

	"ningharness/contract"
)

func TestSortTreeChildrenDirsFirst(t *testing.T) {
	nodes := []contract.TreeNode{
		{Name: "第一章.md", RelPath: "章节/第一章.md"},
		{Name: "章节", RelPath: "章节", IsDir: true},
		{Name: "设定.md", RelPath: "设定.md"},
	}
	sortTreeChildren(nodes)
	if nodes[0].Name != "章节" || nodes[1].Name != "第一章.md" || nodes[2].Name != "设定.md" {
		t.Fatalf("unexpected order: %+v", nodes)
	}
}
