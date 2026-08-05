package job

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	writeTargetLineRe = regexp.MustCompile(`(?m)^[ \t]*- \*\*本轮只写\*\*：` + "`[^`]*`" + `[^\n]*\n?`)
	writeTargetBlockRe = regexp.MustCompile(`(?m)^[ \t]*- \*\*本轮只写\*\*：[ \t]*\n(?:[ \t]*- ` + "`[^`]+`" + `[ \t]*\n?)+`)
)

// WriteTargetLine 前馈单行「本轮只写」（告知写哪；验收只汇总实际写盘路径，不强制再轮）。
func WriteTargetLine(rel string) string {
	rel = NormRel(rel)
	if rel == "" {
		return ""
	}
	return fmt.Sprintf("- **本轮只写**：`%s`", rel)
}

// InjectWriteTarget 以 targetRel 为权威落点：去掉旧「本轮只写」单行/多行块后置顶注入。
// FeedExtra 与 TargetRel 并存时，执行节以 TargetRel 为准，避免写路径解析读到第一条旧路径。
func InjectWriteTarget(feed, targetRel string) string {
	feed = strings.TrimSpace(feed)
	line := WriteTargetLine(targetRel)
	if line == "" {
		return feed
	}
	if feed != "" {
		feed = writeTargetBlockRe.ReplaceAllString(feed, "")
		feed = writeTargetLineRe.ReplaceAllString(feed, "")
		feed = strings.TrimSpace(feed)
	}
	if feed == "" {
		return line
	}
	if strings.Contains(feed, line) {
		return feed
	}
	return line + "\n" + feed
}
