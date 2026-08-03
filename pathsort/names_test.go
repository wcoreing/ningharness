package pathsort

import (
	"sort"
	"testing"
)

func TestChapterSortKey(t *testing.T) {
	cases := map[string]int{
		"第一章.md":           1,
		"第一章_三味火坑.md":      1,
		"第二章.md":           2,
		"第七章.md":           7,
		"第九章.md":           9,
		"第二十章.md":          20,
		"第二十一章_口器.md":      21,
		"第二十七章_铡刀落下.md":    27,
		"第12章.md":           12,
		"第12章-尾声.md":        12,
		"chapter-3.md":       3,
		"chapter-3_intro.md": 3,
	}
	for name, want := range cases {
		got, ok := ChapterSortKey(name)
		if !ok || got != want {
			t.Fatalf("%s: got %d ok=%v want %d", name, got, ok, want)
		}
	}
}

func TestCompareNameChapters(t *testing.T) {
	names := []string{
		"第九章_破布里的剑.md",
		"第一章_三味火坑.md",
		"第二十七章_铡刀落下.md",
		"第三章_炉中山海.md",
	}
	want := []string{
		"第一章_三味火坑.md",
		"第三章_炉中山海.md",
		"第九章_破布里的剑.md",
		"第二十七章_铡刀落下.md",
	}
	sort.Slice(names, func(i, j int) bool { return CompareName(names[i], names[j]) < 0 })
	for i, n := range want {
		if names[i] != n {
			t.Fatalf("index %d: got %s want %s", i, names[i], n)
		}
	}
}

func TestLessRelPathChapters(t *testing.T) {
	rels := []string{
		"章节/第九章_破布里的剑.md",
		"章节/第一章_三味火坑.md",
		"章节/第二十七章_铡刀落下.md",
		"章节/第三章_炉中山海.md",
	}
	sort.Slice(rels, func(i, j int) bool { return LessRelPath(rels[i], rels[j]) })
	want := []string{
		"章节/第一章_三味火坑.md",
		"章节/第三章_炉中山海.md",
		"章节/第九章_破布里的剑.md",
		"章节/第二十七章_铡刀落下.md",
	}
	for i, r := range want {
		if rels[i] != r {
			t.Fatalf("index %d: got %s want %s", i, rels[i], r)
		}
	}
}
