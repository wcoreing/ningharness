// write_file 参数解析：大段正文勿依赖脆弱 JSON。
// 支持纯文本（首行路径 + 空行 + 正文）与 JSON；JSON 失败时尝试引号/换行修复与截断恢复。
package toolhost

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// ParseWriteFile 解析 write_file 调用参数，返回相对路径与全文。
func ParseWriteFile(raw string) (path, content string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return "", "", fmt.Errorf("write_file: 参数为空")
	}

	if path, content, ok := tryJSONWrite(raw); ok {
		return path, content, nil
	}
	if path, content, ok := tryPlainWrite(raw); ok {
		return path, content, nil
	}
	if path, content, ok := recoverTruncatedWriteJSON(raw); ok {
		return path, content, nil
	}

	hint := "请改用纯文本参数：第一行相对路径，空行，其后全文（勿包 JSON）；或完整 JSON {\"rel_path\",\"content\"}"
	if strings.HasPrefix(raw, "{") && !strings.HasSuffix(strings.TrimSpace(raw), "}") {
		return "", "", fmt.Errorf("write_file: JSON 参数被截断（unexpected end）。%s", hint)
	}
	return "", "", fmt.Errorf("write_file: 无法解析参数。%s", hint)
}

func tryJSONWrite(raw string) (path, content string, ok bool) {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		repaired := repairToolJSON(raw, "content", "rel_path", "path", "body", "text", "file")
		if err2 := json.Unmarshal([]byte(repaired), &m); err2 != nil {
			return "", "", false
		}
	}
	path = strField(m, "rel_path", "path", "file", "file_path")
	content = strFieldKeep(m, "content", "body", "text")
	if path == "" {
		return "", "", false
	}
	// content 允许空串（清空文件）
	return path, content, true
}

func tryPlainWrite(raw string) (path, content string, ok bool) {
	// JSON 开头不走纯文本（交给 JSON / 截断恢复）
	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		return "", "", false
	}
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return "", "", false
	}
	head := strings.TrimSpace(lines[0])
	head = strings.TrimPrefix(head, "\ufeff")
	path = plainPathLine(head)
	if path == "" || strings.ContainsAny(path, "\n\r") {
		return "", "", false
	}
	rest := lines[1:]
	// 可选空行分隔
	if len(rest) > 0 && strings.TrimSpace(rest[0]) == "" {
		rest = rest[1:]
	}
	content = strings.Join(rest, "\n")
	// 纯文本至少要有路径；正文可空
	return path, content, true
}

func plainPathLine(head string) string {
	lower := strings.ToLower(head)
	for _, prefix := range []string{"rel_path:", "path:", "file:", "file_path:"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(head[len(prefix):])
		}
	}
	// 拒绝明显不是路径的首行
	if strings.Contains(head, " ") && !strings.Contains(head, "/") && !strings.Contains(head, ".") {
		return ""
	}
	return head
}

// recoverTruncatedWriteJSON 从被截断的 {"rel_path":"...","content":"... 中尽量捞出路径与正文。
func recoverTruncatedWriteJSON(raw string) (path, content string, ok bool) {
	if !strings.Contains(raw, `"content"`) && !strings.Contains(raw, `"body"`) {
		return "", "", false
	}
	path = extractJSONStringField(raw, "rel_path")
	if path == "" {
		path = extractJSONStringField(raw, "path")
	}
	if path == "" {
		path = extractJSONStringField(raw, "file_path")
	}
	if path == "" {
		return "", "", false
	}
	content, ok = extractJSONStringFieldAllowTruncated(raw, "content")
	if !ok {
		content, ok = extractJSONStringFieldAllowTruncated(raw, "body")
	}
	if !ok {
		return "", "", false
	}
	return path, content, true
}

func extractJSONStringField(raw, field string) string {
	s, ok := extractJSONStringFieldAllowTruncated(raw, field)
	if !ok {
		return ""
	}
	return s
}

func extractJSONStringFieldAllowTruncated(raw, field string) (string, bool) {
	prefixes := []string{`"` + field + `": "`, `"` + field + `":"`}
	idx := -1
	prefixLen := 0
	for _, p := range prefixes {
		if at := strings.Index(raw, p); at >= 0 {
			idx = at
			prefixLen = len(p)
			break
		}
	}
	if idx < 0 {
		return "", false
	}
	start := idx + prefixLen
	var b strings.Builder
	i := start
	closed := false
	for i < len(raw) {
		c := raw[i]
		if c == '\\' {
			if i+1 >= len(raw) {
				// 截断在转义符上：丢弃残缺转义
				break
			}
			esc := raw[i+1]
			switch esc {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '"', '\\', '/':
				b.WriteByte(esc)
			case 'u':
				// 不完整 \uXXXX 时停止
				if i+5 >= len(raw) {
					break
				}
				b.WriteByte('\\')
				b.WriteString(raw[i+1 : i+6])
				i += 6
				continue
			default:
				b.WriteByte(esc)
			}
			i += 2
			continue
		}
		if c == '"' {
			j := i + 1
			for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
				j++
			}
			if j >= len(raw) || raw[j] == ',' || raw[j] == '}' {
				closed = true
				break
			}
			// 字段内未转义引号：保留为字面 "
			b.WriteByte('"')
			i++
			continue
		}
		b.WriteByte(c)
		i++
	}
	out := b.String()
	// 截断恢复：未闭合也接受（有正文即可）
	if !closed && out == "" {
		return "", false
	}
	return out, true
}

func strField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func strFieldKeep(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// repairToolJSON 修复指定字符串字段内未转义引号与字面换行（对齐稿舍 domain.UnmarshalToolArguments）。
func repairToolJSON(raw string, fields ...string) string {
	out := raw
	for _, field := range fields {
		out = repairJSONFieldString(out, field)
	}
	return out
}

func repairJSONFieldString(raw, field string) string {
	prefixes := []string{`"` + field + `": "`, `"` + field + `":"`}
	s := raw
	changed := false
	for _, prefix := range prefixes {
		searchFrom := 0
		for {
			rel := strings.Index(s[searchFrom:], prefix)
			if rel < 0 {
				break
			}
			idx := searchFrom + rel
			contentStart := idx + len(prefix)
			fixed, contentEnd, ok := repairJSONStringContent(s, contentStart)
			if !ok {
				searchFrom = contentStart + 1
				continue
			}
			original := s[contentStart:contentEnd]
			if fixed == original {
				searchFrom = contentEnd + 1
				continue
			}
			s = s[:contentStart] + fixed + s[contentEnd:]
			changed = true
			searchFrom = contentStart + len(fixed)
		}
	}
	if !changed {
		return raw
	}
	return s
}

func repairJSONStringContent(s string, contentStart int) (fixed string, end int, ok bool) {
	var b strings.Builder
	i := contentStart
	for i < len(s) {
		c := s[i]
		if c == '\\' {
			if i+1 >= len(s) {
				return "", contentStart, false
			}
			b.WriteByte(c)
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if c == '"' {
			j := i + 1
			for j < len(s) && unicode.IsSpace(rune(s[j])) {
				j++
			}
			if j >= len(s) || s[j] == ',' || s[j] == '}' || s[j] == ']' {
				return b.String(), i, true
			}
			b.WriteString(`\"`)
			i++
			continue
		}
		if c == '\n' {
			b.WriteString(`\n`)
			i++
			continue
		}
		if c == '\r' {
			if i+1 < len(s) && s[i+1] == '\n' {
				b.WriteString(`\n`)
				i += 2
			} else {
				b.WriteString(`\n`)
				i++
			}
			continue
		}
		if c == '\t' {
			b.WriteString(`\t`)
			i++
			continue
		}
		b.WriteByte(c)
		i++
	}
	return "", contentStart, false
}

// EncodeWriteFileJSON 将 path/content 规范成 Invoke 用的 JSON（内部落盘路径统一）。
func EncodeWriteFileJSON(path, content string) string {
	b, err := json.Marshal(map[string]string{
		"rel_path": path,
		"content":  content,
	})
	if err != nil {
		return ""
	}
	return string(b)
}
