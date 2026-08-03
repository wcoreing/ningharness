package task

import (
	"encoding/json"
	"strings"

	"ningharness/history"
)

// StepDiffLine 写盘对照行（DTO；正文在 resource kind=diff）。
type StepDiffLine struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// StepDiff 写盘 diff 元数据（DTO；行内容外置 resource）。
type StepDiff struct {
	Path  string         `json:"path"`
	Add   int            `json:"add"`
	Del   int            `json:"del"`
	Lines []StepDiffLine `json:"lines,omitempty"`
}

// Step UI 回合步骤 DTO：由 history_message 推导；thinking 全文；tool 仅 summary。
type Step struct {
	Kind        string    `json:"kind"`
	Text        string    `json:"text,omitempty"`
	Name        string    `json:"name,omitempty"`
	CallID      string    `json:"callId,omitempty"`
	Args        string    `json:"args,omitempty"`
	Result      string    `json:"result,omitempty"`
	ResourceIDs []int64   `json:"resourceIds,omitempty"`
	OK          bool      `json:"ok"`
	Done        bool      `json:"done"`
	Diff        *StepDiff `json:"diff,omitempty"`
}

// StepsFromHistory 从 history_message（含 thinking / assistant tool_calls / tool）推导 UI 步骤。
func StepsFromHistory(msgs []history.Msg) []Step {
	if len(msgs) == 0 {
		return nil
	}
	var out []Step
	openByCall := map[string]int{}
	for _, m := range msgs {
		switch strings.TrimSpace(m.Role) {
		case "thinking":
			text := m.Content
			if strings.TrimSpace(text) == "" {
				continue
			}
			n := len(out)
			if n > 0 && out[n-1].Kind == "thinking" {
				out[n-1].Text = out[n-1].Text + text
				continue
			}
			out = append(out, Step{Kind: "thinking", Text: text})
		case "assistant":
			raw := strings.TrimSpace(m.ToolCallsJSON)
			if raw == "" {
				continue
			}
			var calls []history.ToolCallSpec
			if err := json.Unmarshal([]byte(raw), &calls); err != nil {
				continue
			}
			for _, c := range calls {
				name := strings.TrimSpace(c.Name)
				if name == "" {
					continue
				}
				callID := strings.TrimSpace(c.ID)
				if callID == "" {
					callID = name
				}
				idx := len(out)
				out = append(out, Step{
					Kind: "tool", Name: name, CallID: callID, Args: c.Arguments,
					ResourceIDs: history.MergeResourceIDs(c.ResourceIDs),
					OK: true, Done: false,
				})
				openByCall[callID] = idx
			}
		case "tool":
			callID := strings.TrimSpace(m.ToolCallID)
			text := m.Content
			errish := LooksFailed(text)
			if idx, ok := openByCall[callID]; ok {
				cur := out[idx]
				cur.Result = text
				cur.ResourceIDs = history.MergeResourceIDs(cur.ResourceIDs, history.DecodeResourceIDs(m.ResourceIDsJSON))
				cur.OK = cur.OK && !errish
				cur.Done = true
				out[idx] = cur
				delete(openByCall, callID)
				continue
			}
			out = append(out, Step{
				Kind: "tool", CallID: callID, Result: text,
				ResourceIDs: history.DecodeResourceIDs(m.ResourceIDsJSON),
				OK: !errish, Done: true,
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
