package task

import "strings"

// FormatToolDigestLines 交付足迹行（仅供 ContextMessages 摘要，禁止写入助理正文）。
func FormatToolDigestLines(tools []ToolCall) []string {
	if len(tools) == 0 {
		return nil
	}
	parts := make([]string, 0, len(tools))
	for _, t := range tools {
		n := strings.TrimSpace(t.Name)
		if n == "" {
			continue
		}
		s := n
		if p := strings.TrimSpace(t.Path); p != "" {
			s += "→" + p
		}
		if !t.OK {
			s += "(fail)"
		}
		parts = append(parts, s)
	}
	return parts
}

// CleanAssistantText 去掉正文里的 `[Desk tools: …]` 行与写路径纠偏注。
func CleanAssistantText(body string) string {
	body = strings.ReplaceAll(body, "\n\n（系统：本轮未见写盘成功回执，上文未落盘。）", "")
	body = strings.ReplaceAll(body, "（系统：本轮未见写盘成功回执，上文未落盘。）", "")
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	prevBlank := false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "[Desk tools:") && strings.HasSuffix(t, "]") {
			continue
		}
		blank := t == ""
		if blank && prevBlank {
			continue
		}
		out = append(out, ln)
		prevBlank = blank
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
