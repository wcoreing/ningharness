package workspace

import (
	"fmt"
	"strings"

	"ningharness/docwords"
)

// ApplyEdit 在正文中替换 old→new。replaceAll=false 时 old 必须恰好出现一次。
func ApplyEdit(body, old, new string, replaceAll bool) (string, int, error) {
	if old == "" {
		return "", 0, fmt.Errorf("old_string 不能为空")
	}
	n := strings.Count(body, old)
	if n == 0 {
		return "", 0, fmt.Errorf("old_string 未在文件中找到；请 read_file 核对原文后重试")
	}
	if n > 1 && !replaceAll {
		return "", n, fmt.Errorf("old_string 出现 %d 次；请扩大上下文使唯一，或设 replace_all=true", n)
	}
	var out string
	if replaceAll {
		out = strings.ReplaceAll(body, old, new)
	} else {
		out = strings.Replace(body, old, new, 1)
	}
	return out, n, nil
}

// FormatEditOK edit 成功回执（writeID 可选，便于雷达对账）。
func FormatEditOK(rel string, replacements int, content, writeID string) string {
	n := docwords.Count(content)
	var msg string
	if replacements == 1 {
		msg = fmt.Sprintf("Successfully edited '%s'（1 处 · %d 字）", rel, n)
	} else {
		msg = fmt.Sprintf("Successfully edited '%s'（%d 处 · %d 字）", rel, replacements, n)
	}
	if wid := strings.TrimSpace(writeID); wid != "" {
		msg += " writeId=" + wid
	}
	return msg
}
