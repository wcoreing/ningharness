package skill

import (
	"fmt"
	"regexp"
	"strings"
)

var leadingSkillCmdRE = regexp.MustCompile(`^/([^\s]+)(?:\s+([\s\S]*))?$`)

// ResolveLeadingCommand 解析消息开头 /id 或 /name，返回正文与 skill id 列表。
// list 通常为 ListEnabled 结果；未命中时原样返回 message。
func ResolveLeadingCommand(message string, list []Info) (rest string, skillIDs []string) {
	trimmed := strings.TrimSpace(message)
	m := leadingSkillCmdRE.FindStringSubmatch(trimmed)
	if m == nil {
		return message, nil
	}
	token := strings.TrimSpace(m[1])
	rest = strings.TrimSpace(m[2])
	hit := resolveSkillToken(token, list)
	if hit == nil {
		return message, nil
	}
	if rest == "" {
		rest = fmt.Sprintf("请按技能「%s」执行。", hit.Name)
	}
	return rest, []string{hit.ID}
}

func resolveSkillToken(token string, list []Info) *Info {
	t := strings.TrimSpace(token)
	if t == "" || len(list) == 0 {
		return nil
	}
	for i := range list {
		s := &list[i]
		if s.ID == t || strings.EqualFold(s.ID, t) || s.Name == t || strings.EqualFold(s.Name, t) {
			return s
		}
	}
	q := strings.ToLower(t)
	var partial []*Info
	for i := range list {
		s := &list[i]
		if strings.Contains(strings.ToLower(s.ID), q) ||
			strings.Contains(strings.ToLower(s.Name), q) ||
			strings.Contains(strings.ToLower(s.Description), q) {
			partial = append(partial, s)
		}
	}
	if len(partial) == 1 {
		return partial[0]
	}
	if len(partial) > 1 {
		var starts []*Info
		for _, s := range partial {
			if strings.HasPrefix(strings.ToLower(s.ID), q) || strings.HasPrefix(strings.ToLower(s.Name), q) {
				starts = append(starts, s)
			}
		}
		if len(starts) == 1 {
			return starts[0]
		}
		best := partial[0]
		for _, s := range partial[1:] {
			if len(s.ID) < len(best.ID) {
				best = s
			}
		}
		return best
	}
	return nil
}
