package lesson

import (
	"strconv"
	"strings"
	"time"
)

// parseMarkdownEntries 解析 MEMORY / USER / LESSONS MD 节。
func parseMarkdownEntries(md, scope, skillID string) []Entry {
	md = strings.TrimSpace(md)
	if md == "" {
		return nil
	}
	parts := strings.Split(md, "\n## ")
	var out []Entry
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if i == 0 {
			lower := strings.ToLower(p)
			if strings.HasPrefix(lower, "# lessons") || strings.HasPrefix(lower, "# memory") || strings.HasPrefix(lower, "# user") {
				if idx := strings.Index(p, "\n"); idx >= 0 {
					rest := strings.TrimSpace(p[idx+1:])
					if rest != "" && !strings.HasPrefix(rest, "#") {
						out = append(out, Entry{
							Scope: scope, SkillID: skillID,
							Anchors:     Anchors{SkillID: skillID},
							Body:        rest,
							Status:      StatusActive,
							AckedAtMs:   time.Now().UnixMilli(),
							CreatedAtMs: time.Now().UnixMilli(),
						})
					}
				}
				continue
			}
		}
		title, body, _ := strings.Cut(p, "\n")
		title = strings.TrimSpace(title)
		body = strings.TrimSpace(body)
		status := StatusActive
		created := int64(0)
		if base, suf, ok := strings.Cut(title, " · "); ok {
			suf = strings.ToLower(strings.TrimSpace(suf))
			suf = strings.TrimPrefix(suf, "status:")
			switch suf {
			case StatusActive, StatusSuperseded, StatusExpired:
				status = suf
				title = strings.TrimSpace(base)
			}
			if t, err := time.ParseInLocation("2006-01-02 15:04", title, time.Local); err == nil {
				created = t.UnixMilli()
			} else if t, err := time.ParseInLocation("2006-01-02", title, time.Local); err == nil {
				created = t.UnixMilli()
			}
		}
		e := Entry{
			Scope: scope, SkillID: skillID, Anchors: Anchors{SkillID: skillID},
			Status: status, CreatedAtMs: created,
		}
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
				case "source_session":
					e.SourceSessionKey = val
					continue
				case "supersedes":
					e.SupersedesID = val
					continue
				case "acked_at":
					if n, err := strconv.ParseInt(val, 10, 64); err == nil {
						e.AckedAtMs = n
					}
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
		if e.CreatedAtMs == 0 {
			e.CreatedAtMs = time.Now().UnixMilli()
		}
		if e.AckedAtMs == 0 {
			e.AckedAtMs = e.CreatedAtMs
		}
		out = append(out, e)
	}
	return out
}
