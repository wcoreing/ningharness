package docwords

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	fenceRe      = regexp.MustCompile("(?s)```.*?```")
	linkRe       = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	tableSepRe   = regexp.MustCompile(`^\s*\|?[\s:|-]+\|[\s:|-]*\|?\s*$`)
	headingRe    = regexp.MustCompile(`^\s{0,3}#{1,6}\s+`)
	blockquoteRe = regexp.MustCompile(`^\s{0,3}>\s?`)
	listRe       = regexp.MustCompile(`^\s*([-*+]|\d+\.)\s+`)
	mdMarkRe     = regexp.MustCompile(`[*_\x60~|]+`)
)

// Count 稿面字数：人眼可见正文近似值（去 MD 壳与空白），供树/写盘回执共用。
// 不含围栏源码（mermaid 导出走图）、表格分隔行、常见标记与空白；与 Word「字数」仍可能因目录域/图片略有偏差。
func Count(s string) int {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = fenceRe.ReplaceAllString(s, "\n")
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if tableSepRe.MatchString(line) {
			continue
		}
		line = headingRe.ReplaceAllString(line, "")
		line = blockquoteRe.ReplaceAllString(line, "")
		line = listRe.ReplaceAllString(line, "")
		line = linkRe.ReplaceAllString(line, "$1")
		line = mdMarkRe.ReplaceAllString(line, "")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	var n int
	for _, r := range b.String() {
		if unicode.IsSpace(r) {
			continue
		}
		n++
	}
	return n
}

// CountRaw 源文全量 rune（含空白与标记）；仅调试/对比用。
func CountRaw(s string) int {
	return utf8.RuneCountInString(strings.TrimSpace(s))
}
