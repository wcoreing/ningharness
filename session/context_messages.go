package session

import (
	"fmt"
	"strings"
)

// ContextMessage 喂给模型的一条进模上下文（与磁盘 Message 分离；可截断/摘要）。
type ContextMessage struct {
	Role    string
	Content string
}

// ToolDigestLookup 按 taskId 取交付足迹（write_file→path …）。真源在 runs，不进助理正文。
type ToolDigestLookup func(taskID string) []string

// ContextBudget 构造 ContextMessages 预算（磁盘 sessions.json 仍全量保留）。
type ContextBudget struct {
	// RecentKeep 最近原文条数（默认 12）。
	RecentKeep int
	// MaxMsgRunes 单条最大 rune（默认 2400）。
	MaxMsgRunes int
	// OlderUserKeep 更早历史里保留的 user 要点条数（默认 8）。
	OlderUserKeep int
	// OlderUserRunes 每条旧 user 摘要 rune（默认 160）。
	OlderUserRunes int
	// OlderAssistKeep 折叠区保留的助理结论条数（默认 4）。
	OlderAssistKeep int
	// OlderAssistRunes 每条助理结论 rune（默认 120）。
	OlderAssistRunes int
	// OlderToolKeep 折叠区保留的工具足迹条数（默认 8）。
	OlderToolKeep int
}

// DefaultContextBudget 默认有界 ContextMessages。
func DefaultContextBudget() ContextBudget {
	return ContextBudget{
		RecentKeep:       12,
		MaxMsgRunes:      2400,
		OlderUserKeep:    8,
		OlderUserRunes:   160,
		OlderAssistKeep:  4,
		OlderAssistRunes: 120,
		OlderToolKeep:    8,
	}
}

func (b ContextBudget) normalize() ContextBudget {
	if b.RecentKeep <= 0 {
		b.RecentKeep = 12
	}
	if b.MaxMsgRunes <= 0 {
		b.MaxMsgRunes = 2400
	}
	if b.OlderUserKeep <= 0 {
		b.OlderUserKeep = 8
	}
	if b.OlderUserRunes <= 0 {
		b.OlderUserRunes = 160
	}
	if b.OlderAssistKeep <= 0 {
		b.OlderAssistKeep = 4
	}
	if b.OlderAssistRunes <= 0 {
		b.OlderAssistRunes = 120
	}
	if b.OlderToolKeep <= 0 {
		b.OlderToolKeep = 8
	}
	return b
}

// BuildContextMessages 从全量会话消息构造有界 ContextMessages（不含本轮尚未 Append 的 user）。
// 助理正文不含工具行；交付足迹经 lookup(taskId) 进 system，避免模型仿写 `[Desk tools:…]`。
func BuildContextMessages(msgs []Message, budget ContextBudget, lookup ToolDigestLookup) []ContextMessage {
	b := budget.normalize()
	n := len(msgs)
	if n == 0 {
		return nil
	}
	start := 0
	if n > b.RecentKeep {
		start = n - b.RecentKeep
	}
	var out []ContextMessage
	if start > 0 {
		if sum := summarizeOlder(msgs[:start], b, lookup); sum != "" {
			out = append(out, ContextMessage{Role: "system", Content: sum})
		}
	}
	if dig := formatDigestBlock("近期交付足迹", collectDigests(msgs[start:], b.OlderToolKeep, lookup)); dig != "" {
		out = append(out, ContextMessage{Role: "system", Content: dig})
	}
	for _, m := range msgs[start:] {
		role := strings.TrimSpace(m.Role)
		if role == "" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if role == "assistant" {
			content = stripAllToolFootprintLines(content)
		}
		content = trimRunes(content, b.MaxMsgRunes)
		if content == "" {
			continue
		}
		out = append(out, ContextMessage{Role: role, Content: content})
	}
	return out
}

func summarizeOlder(older []Message, b ContextBudget, lookup ToolDigestLookup) string {
	if len(older) == 0 {
		return ""
	}
	users := collectFromTail(older, "user", b.OlderUserKeep, b.OlderUserRunes, false)
	assists := collectFromTail(older, "assistant", b.OlderAssistKeep, b.OlderAssistRunes, true)
	tools := collectDigests(older, b.OlderToolKeep, lookup)

	var sb strings.Builder
	fmt.Fprintf(&sb, "（工作记忆摘要：更早 %d 条已折叠；磁盘会话仍完整。查往事用 search_session。）\n", len(older))
	sb.WriteString("旧用户要点：\n")
	if len(users) == 0 {
		sb.WriteString("- （无）\n")
	} else {
		for _, u := range users {
			fmt.Fprintf(&sb, "- %s\n", u)
		}
	}
	if len(tools) > 0 {
		sb.WriteString("折叠区交付足迹：\n")
		for _, t := range tools {
			fmt.Fprintf(&sb, "- %s\n", t)
		}
	}
	if len(assists) > 0 {
		sb.WriteString("折叠区助理结论：\n")
		for _, a := range assists {
			fmt.Fprintf(&sb, "- %s\n", a)
		}
	}
	return strings.TrimSpace(sb.String())
}

func formatDigestBlock(title string, digests []string) string {
	if len(digests) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s（系统记录，勿复述到回复正文）：\n", title)
	for _, d := range digests {
		fmt.Fprintf(&sb, "- %s\n", d)
	}
	return strings.TrimSpace(sb.String())
}

func collectFromTail(older []Message, role string, keep, maxRunes int, stripFootprint bool) []string {
	if keep <= 0 {
		return nil
	}
	items := make([]string, 0, keep)
	for i := len(older) - 1; i >= 0 && len(items) < keep; i-- {
		if older[i].Role != role {
			continue
		}
		raw := strings.TrimSpace(older[i].Content)
		if stripFootprint {
			raw = stripAllToolFootprintLines(raw)
		}
		line := trimRunes(raw, maxRunes)
		if line == "" {
			continue
		}
		items = append(items, line)
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items
}

func collectDigests(msgs []Message, keep int, lookup ToolDigestLookup) []string {
	if keep <= 0 {
		return nil
	}
	out := make([]string, 0, keep)
	seen := map[string]bool{}
	for i := len(msgs) - 1; i >= 0 && len(out) < keep; i-- {
		if msgs[i].Role != "assistant" {
			continue
		}
		var dig string
		if lookup != nil {
			if id := strings.TrimSpace(msgs[i].TaskID); id != "" {
				if lines := lookup(id); len(lines) > 0 {
					dig = strings.Join(lines, ", ")
				}
			}
		}
		if dig == "" {
			// 旧数据：正文曾内嵌 [Desk tools:…]
			if fp := extractToolsFootprint(msgs[i].Content); fp != "" {
				dig = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(fp, "[Desk tools:"), "]"))
			}
		}
		if dig == "" || dig == "none" || seen[dig] {
			continue
		}
		seen[dig] = true
		out = append(out, trimRunes(dig, 200))
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func extractToolsFootprint(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "[Desk tools:") {
		return ""
	}
	end := strings.Index(content, "]")
	if end < 0 {
		return ""
	}
	return trimRunes(content[:end+1], 200)
}

// stripAllToolFootprintLines 去掉正文中所有 `[Desk tools: …]` 行（含中段仿写）。
func stripAllToolFootprintLines(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "[Desk tools:") && strings.HasSuffix(t, "]") {
			continue
		}
		out = append(out, ln)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
