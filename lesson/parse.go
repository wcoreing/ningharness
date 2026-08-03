package lesson

import (
	"strings"
)

type legacyEntry struct {
	ID           string
	Body         string
	Status       string
	SourceTaskID string
	ParentTaskID string
	SupersedesID string
}

func parseLegacyMarkdown(md string) []legacyEntry {
	md = strings.TrimSpace(md)
	if md == "" {
		return nil
	}
	parts := strings.Split(md, "\n## ")
	var out []legacyEntry
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i == 0 && (strings.HasPrefix(p, "# LESSONS") || strings.HasPrefix(strings.ToLower(p), "# lessons")) {
			rest := strings.TrimSpace(strings.TrimPrefix(p, "# LESSONS"))
			rest = strings.TrimSpace(strings.TrimPrefix(rest, "# Lessons"))
			if rest != "" && !strings.Contains(rest, "\n## ") && !strings.HasPrefix(rest, "#") {
				out = append(out, legacyEntry{Body: rest, Status: StatusActive})
			}
			continue
		}
		title, body, _ := strings.Cut(p, "\n")
		title = strings.TrimSpace(title)
		body = strings.TrimSpace(body)
		status := StatusActive
		if base, suf, ok := strings.Cut(title, " · "); ok {
			suf = strings.ToLower(strings.TrimSpace(suf))
			suf = strings.TrimPrefix(suf, "status:")
			switch suf {
			case StatusActive, StatusSuperseded, StatusExpired:
				status = suf
				title = strings.TrimSpace(base)
			}
		}
		e := legacyEntry{Status: status}
		lines := strings.Split(body, "\n")
		var bodyLines []string
		for _, line := range lines {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "<!-- lesson:") && strings.HasSuffix(trim, "-->") {
				id := strings.TrimSuffix(strings.TrimPrefix(trim, "<!-- lesson:"), "-->")
				e.ID = strings.TrimSpace(id)
				continue
			}
			if k, v, ok := strings.Cut(trim, ":"); ok {
				key := strings.ToLower(strings.TrimSpace(k))
				val := strings.TrimSpace(v)
				switch key {
				case "source_task":
					e.SourceTaskID = val
					continue
				case "parent_task":
					e.ParentTaskID = val
					continue
				case "supersedes":
					e.SupersedesID = val
					continue
				}
			}
			bodyLines = append(bodyLines, line)
		}
		e.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
		if e.Body == "" && title != "" {
			e.Body = title
		}
		if e.Body == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}
