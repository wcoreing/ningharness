package workspace

import (
	"fmt"

	"github.com/wcoreing/ningharness/protocol"
)

// AgentTreeMaxChildren MCP list_tree 单目录展示上限；超出折叠（UI ListTree 仍全量）。
const AgentTreeMaxChildren = 40

// CompactListingForAgent 裁剪过宽目录，避免参考料成百文件撑爆模型上下文。
// 目录 WordCount 仍是折叠前合计；末尾一条占位说明另 +N。
func CompactListingForAgent(in TreeListing, maxChildren int) TreeListing {
	if maxChildren <= 0 {
		maxChildren = AgentTreeMaxChildren
	}
	nodes, cut := compactTreeNodes(in.Nodes, maxChildren)
	return TreeListing{Nodes: nodes, Truncated: cut || in.Truncated}
}

func compactTreeNodes(nodes []protocol.TreeNode, maxChildren int) ([]protocol.TreeNode, bool) {
	if len(nodes) == 0 {
		return nodes, false
	}
	out := make([]protocol.TreeNode, len(nodes))
	cut := false
	for i := range nodes {
		n := nodes[i]
		var childCut bool
		n.Children, childCut = compactTreeNodes(n.Children, maxChildren)
		if childCut {
			cut = true
		}
		if n.IsDir && len(n.Children) > maxChildren {
			cut = true
			omitted := len(n.Children) - maxChildren
			kept := append([]protocol.TreeNode{}, n.Children[:maxChildren]...)
			kept = append(kept, protocol.TreeNode{
				RelPath: n.RelPath + "/…",
				Name:    fmt.Sprintf("…另 +%d 项（点名路径 / grep）", omitted),
			})
			n.Children = kept
		}
		out[i] = n
	}
	return out, cut
}
