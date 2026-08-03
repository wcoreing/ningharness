package history

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var resourceIDTagRe = regexp.MustCompile(`〔resource#(\d+)〕|resource#(\d+)`)

// EncodeResourceIDs 序列化 history_message.resource_ids_json / tool 行索引。
func EncodeResourceIDs(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	out := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return ""
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(raw)
}

// DecodeResourceIDs 解析 resource_ids_json。
func DecodeResourceIDs(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			out = append(out, id)
		}
	}
	return out
}

// WireTool 进模 tool 正文：content + 按需 recall 提示（resource_ids 结构化存储，不污染 arguments）。
func WireTool(m Msg) string {
	content := strings.TrimSpace(m.Content)
	ids := DecodeResourceIDs(m.ResourceIDsJSON)
	if len(ids) == 0 {
		return content
	}
	if strings.Contains(content, "recall_resource") {
		return content
	}
	var b strings.Builder
	if content != "" {
		b.WriteString(content)
		b.WriteByte('\n')
	}
	if len(ids) == 1 {
		fmt.Fprintf(&b, "recall_resource resource_id=%d", ids[0])
	} else {
		fmt.Fprintf(&b, "recall_resource resource_ids=%s", EncodeResourceIDs(ids))
	}
	return b.String()
}

// CollectResourceIDs 从 history 行汇总 resource 索引（结构化字段 + 旧 content 兼容）。
func CollectResourceIDs(msgs []Msg) []int64 {
	seen := map[int64]struct{}{}
	var out []int64
	add := func(ids []int64) {
		for _, id := range ids {
			if id <= 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	addFromText := func(s string) {
		for _, m := range resourceIDTagRe.FindAllStringSubmatch(s, -1) {
			var id int64
			for i := 1; i < len(m); i++ {
				if m[i] == "" {
					continue
				}
				if _, err := fmt.Sscanf(m[i], "%d", &id); err == nil && id > 0 {
					add([]int64{id})
					break
				}
			}
		}
	}
	for _, m := range msgs {
		add(DecodeResourceIDs(m.ResourceIDsJSON))
		addFromText(m.Content)
		raw := strings.TrimSpace(m.ToolCallsJSON)
		if raw == "" {
			continue
		}
		var calls []ToolCallSpec
		if err := json.Unmarshal([]byte(raw), &calls); err != nil {
			continue
		}
		for _, c := range calls {
			add(c.ResourceIDs)
			addFromText(c.Arguments)
		}
	}
	return out
}

// MergeResourceIDs 去重合并多组 resource id（call + result 等）。
func MergeResourceIDs(parts ...[]int64) []int64 {
	seen := map[int64]struct{}{}
	var out []int64
	for _, ids := range parts {
		for _, id := range ids {
			if id <= 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}
