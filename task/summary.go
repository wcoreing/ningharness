package task

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var resourceIDRe = regexp.MustCompile(`resource#(\d+)`)

// FormatSummary 给 Agent / MCP 的短摘要：不 dump 全文与大 diff；正文指 recall_resource。
func FormatSummary(rec *Record) string {
	if rec == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "task `%s` · %s · %s", strings.TrimSpace(rec.ID), strings.TrimSpace(rec.Driver), strings.TrimSpace(rec.Status))
	if p := strings.TrimSpace(rec.Purpose); p != "" {
		fmt.Fprintf(&b, " · purpose=%s", p)
	}
	if s := strings.TrimSpace(rec.SessionID); s != "" {
		fmt.Fprintf(&b, " · session=%s", s)
	}
	if j := strings.TrimSpace(rec.JobID); j != "" {
		fmt.Fprintf(&b, " · job=%s", j)
	}
	b.WriteByte('\n')
	if ff := strings.TrimSpace(rec.Feedforward); ff != "" {
		fmt.Fprintf(&b, "feedforward: %d字\n", utf8.RuneCountInString(ff))
	}
	if e := strings.TrimSpace(rec.Error); e != "" {
		fmt.Fprintf(&b, "error: %s\n", trimRunes(e, 240))
	}

	resourceIDs := map[int64]struct{}{}
	for _, id := range rec.ResourceIDs {
		if id > 0 {
			resourceIDs[id] = struct{}{}
		}
	}
	if len(rec.Tools) > 0 {
		b.WriteString("tools:\n")
		for _, t := range rec.Tools {
			mark := "ok"
			if !t.OK {
				mark = "fail"
			}
			line := fmt.Sprintf("- %s [%s]", t.Name, mark)
			if t.Path != "" {
				line += " · " + t.Path
			}
			if d := strings.TrimSpace(t.Detail); d != "" {
				collectResourceIDs(d, resourceIDs)
				line += " · " + trimRunes(stripResourceHints(d), 120)
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	thinkN, toolN := 0, 0
	for _, blk := range rec.Steps {
		switch blk.Kind {
		case "thinking":
			thinkN++
		case "tool":
			toolN++
			collectResourceIDs(blk.Args, resourceIDs)
			collectResourceIDs(blk.Result, resourceIDs)
		}
	}
	if thinkN > 0 || toolN > 0 {
		fmt.Fprintf(&b, "steps: thinking=%d tool=%d（来自 history；工具全文不在此）\n", thinkN, toolN)
	}
	for i := len(rec.Steps) - 1; i >= 0; i-- {
		if rec.Steps[i].Kind != "thinking" {
			continue
		}
		if t := strings.TrimSpace(rec.Steps[i].Text); t != "" {
			fmt.Fprintf(&b, "last_thinking: %s\n", trimRunes(t, 200))
			break
		}
	}

	if len(resourceIDs) > 0 {
		ids := make([]int64, 0, len(resourceIDs))
		for id := range resourceIDs {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		b.WriteString("resources:")
		for _, id := range ids {
			fmt.Fprintf(&b, " #%d", id)
		}
		b.WriteString("\n全文：desk_session call recall_resource resource_id=<上列>\n")
	} else {
		b.WriteString("全文：desk_session call recall_resource（按 tool_call_id / query）\n")
	}
	return strings.TrimSpace(b.String())
}

func collectResourceIDs(s string, into map[int64]struct{}) {
	for _, m := range resourceIDRe.FindAllStringSubmatch(s, -1) {
		if len(m) < 2 {
			continue
		}
		var id int64
		if _, err := fmt.Sscanf(m[1], "%d", &id); err == nil && id > 0 {
			into[id] = struct{}{}
		}
	}
}

func stripResourceHints(s string) string {
	s = strings.ReplaceAll(s, "〔", "")
	s = strings.ReplaceAll(s, "〕", "")
	if i := strings.Index(s, " · 全文 desk_session"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func trimRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}
