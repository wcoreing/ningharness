package history

import (
	"strings"
)

// ContextMessage 进模用的一条上下文（不含本轮 user prompt）。
type ContextMessage struct {
	Role          string // user | assistant | system | tool
	Content       string
	ToolCallID    string // tool 行
	ToolCallsJSON string // assistant 发起的调用 JSON
}

// ToContextMessages 转为进模上下文（含 tool；跳过 thinking）。
func ToContextMessages(msgs []Msg) []ContextMessage {
	out := make([]ContextMessage, 0, len(msgs))
	for _, m := range msgs {
		if strings.TrimSpace(m.Role) == "thinking" {
			continue
		}
		out = append(out, ContextMessage{
			Role:          m.Role,
			Content:       toolContentForContext(m),
			ToolCallID:    m.ToolCallID,
			ToolCallsJSON: m.ToolCallsJSON,
		})
	}
	return out
}

func toolContentForContext(m Msg) string {
	if strings.TrimSpace(m.Role) != "tool" {
		return m.Content
	}
	return WireTool(m)
}
