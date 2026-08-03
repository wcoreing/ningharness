// Package pathsort 文件名/相对路径排序（第N章章号 + 数字自然序）。
package workspace

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	chapterCNRe  = regexp.MustCompile(`^第(.+?)章`)
	chapterNumRe = regexp.MustCompile(`^第(\d+)章`)
	chapterEnRe  = regexp.MustCompile(`(?i)^chapter[-_]?(\d+)`)
)

var cnDigit = map[rune]int{
	'零': 0, '〇': 0, '一': 1, '二': 2, '三': 3, '四': 4,
	'五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
}

var cnUnit = map[rune]int{
	'十': 10, '百': 100, '千': 1000,
}

func parseChineseNumeral(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, true
	}
	section := 0
	number := 0
	hasDigit := false
	for _, r := range s {
		if d, ok := cnDigit[r]; ok {
			number = d
			hasDigit = true
			continue
		}
		if u, ok := cnUnit[r]; ok {
			if number == 0 {
				number = 1
			}
			section += number * u
			number = 0
			hasDigit = true
			continue
		}
		return 0, false
	}
	if !hasDigit {
		return 0, false
	}
	return section + number, true
}

// ChapterSortKey 从文件名解析章号；非章节名 ok=false。
func ChapterSortKey(name string) (int, bool) {
	base := strings.TrimSpace(name)
	if i := strings.LastIndex(base, "."); i >= 0 {
		base = base[:i]
	}
	if m := chapterNumRe.FindStringSubmatch(base); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n, true
	}
	if m := chapterEnRe.FindStringSubmatch(base); len(m) == 2 {
		n, _ := strconv.Atoi(m[1])
		return n, true
	}
	if m := chapterCNRe.FindStringSubmatch(base); len(m) == 2 {
		if n, ok := parseChineseNumeral(m[1]); ok {
			return n, true
		}
	}
	return 0, false
}

// CompareName 比较文件/目录名：双章名按章号；否则数字自然序。
func CompareName(a, b string) int {
	aKey, aOk := ChapterSortKey(a)
	bKey, bOk := ChapterSortKey(b)
	if aOk && bOk && aKey != bKey {
		if aKey < bKey {
			return -1
		}
		return 1
	}
	if naturalLess(a, b) {
		return -1
	}
	if naturalLess(b, a) {
		return 1
	}
	return 0
}

// CompareRelPath 按路径段逐段 CompareName。
func CompareRelPath(a, b string) int {
	pa := splitRel(a)
	pb := splitRel(b)
	n := len(pa)
	if len(pb) < n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		if c := CompareName(pa[i], pb[i]); c != 0 {
			return c
		}
	}
	if len(pa) < len(pb) {
		return -1
	}
	if len(pa) > len(pb) {
		return 1
	}
	return 0
}

// LessRelPath a 是否应排在 b 前。
func LessRelPath(a, b string) bool {
	return CompareRelPath(a, b) < 0
}

func splitRel(rel string) []string {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" || rel == "." {
		return nil
	}
	return strings.Split(rel, "/")
}

func naturalLess(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]
		da, db := ca >= '0' && ca <= '9', cb >= '0' && cb <= '9'
		if da && db {
			var na, nb int
			for ai < len(a) && a[ai] >= '0' && a[ai] <= '9' {
				na = na*10 + int(a[ai]-'0')
				ai++
			}
			for bi < len(b) && b[bi] >= '0' && b[bi] <= '9' {
				nb = nb*10 + int(b[bi]-'0')
				bi++
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		if ca != cb {
			return ca < cb
		}
		ai++
		bi++
	}
	return len(a) < len(b)
}
