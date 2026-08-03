package workspace

import (
	"sort"

	"ningharness/protocol"
	
)

func sortTreeChildren(nodes []protocol.TreeNode) {
	sort.Slice(nodes, func(i, j int) bool {
		ni, nj := nodes[i], nodes[j]
		if ni.IsDir != nj.IsDir {
			return ni.IsDir
		}
		return CompareName(ni.Name, nj.Name) < 0
	})
}
