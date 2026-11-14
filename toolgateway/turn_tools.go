package toolgateway

import (
	"sort"
	"strings"
)

// ToolInfo 已注册工具的可发现元数据（本轮可见集 / External.Tools 裁剪用）。
type ToolInfo struct {
	Name        string    `json:"name"`
	Layer       ToolLayer `json:"layer,omitempty"`
	Description string    `json:"description,omitempty"`
}

// coreToolDescriptions 核工具一句话（与 MCP schema 描述对齐的短版；产品扩展靠 RegisterToolInfo）。
var coreToolDescriptions = map[string]string{
	"list_tree":              "列出工作区目录树",
	"read_file":              "读工作区文本文件",
	"grep":                   "工作区内容搜索",
	"write_file":             "写工作区文件",
	"edit":                   "局部编辑工作区文件",
	"search_session":         "检索会话历史",
	"recall_resource":        "召回外置资源全文",
	"list_skills":            "列出项目 Skill（方法包）",
	"get_skill":              "读取项目 Skill 正文",
	"list_lessons":           "列出 lesson",
	"ack_lesson":             "确认 lesson",
	"list_tasks":             "列出任务",
	"get_task_summary":       "任务摘要",
	"get_task_trace":         "任务 Trace",
	"enqueue_agent_turn":     "入队一轮 Agent",
	"list_queue":             "列出队列",
	"cancel_queue_task":      "取消队列任务",
}

// RegisterToolInfo 为产品扩展工具登记展示元数据（可选；不影响 Invoke）。
func (h *Gateway) RegisterToolInfo(name, description string, layer ToolLayer) {
	if h == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.toolInfo == nil {
		h.toolInfo = map[string]ToolInfo{}
	}
	h.toolInfo[name] = ToolInfo{Name: name, Description: strings.TrimSpace(description), Layer: layer}
}

// RegisteredToolNames 全部已注册工具名（含核工具），排序稳定。
func (h *Gateway) RegisteredToolNames() []string {
	if h == nil {
		return nil
	}
	h.ensureCoreHandlers()
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.handlers))
	for name := range h.handlers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// TurnToolNames 本轮可见工具名。
// 若已 ArmTurnInterceptor 且 allow 非空：只返回 allow∩registered；
// allow 为 nil（未限制）：返回全部已注册。
func (h *Gateway) TurnToolNames() []string {
	if h == nil {
		return nil
	}
	h.ensureCoreHandlers()
	h.mu.Lock()
	allow := h.turnAllow
	handlers := h.handlers
	h.mu.Unlock()

	out := make([]string, 0, len(handlers))
	for name := range handlers {
		if allow != nil && !allow[name] {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// TurnToolInfos 本轮可见工具元数据（名 + 层 + 描述）。
func (h *Gateway) TurnToolInfos() []ToolInfo {
	names := h.TurnToolNames()
	if len(names) == 0 {
		return nil
	}
	h.mu.Lock()
	extra := h.toolInfo
	h.mu.Unlock()
	out := make([]ToolInfo, 0, len(names))
	for _, name := range names {
		info := ToolInfo{Name: name, Layer: CoreToolLayer(name)}
		if d, ok := coreToolDescriptions[name]; ok {
			info.Description = d
		}
		if extra != nil {
			if e, ok := extra[name]; ok {
				if e.Description != "" {
					info.Description = e.Description
				}
				if e.Layer != "" {
					info.Layer = e.Layer
				}
			}
		}
		out = append(out, info)
	}
	return out
}

// ArmTurnTools 按显式工具名武装本轮拦截器（写入 allow）。
func (h *Gateway) ArmTurnTools(purpose string, names []string) {
	allow := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			allow[n] = true
		}
	}
	if len(allow) == 0 {
		h.ArmTurnInterceptor(purpose, nil)
		return
	}
	h.ArmTurnInterceptor(purpose, allow)
}
