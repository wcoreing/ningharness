package workspace

import (
	"sort"

	"ningharness/contract"
	"ningharness/pathsort"
)

func sortTreeChildren(nodes []contract.TreeNode) {
	sort.Slice(nodes, func(i, j int) bool {
		ni, nj := nodes[i], nodes[j]
		if ni.IsDir != nj.IsDir {
			return ni.IsDir
		}
		return pathsort.CompareName(ni.Name, nj.Name) < 0
	})
}
