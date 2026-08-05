package toolgateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ToolHandler 单个命名工具的执行体（与 MCP server 回调签名一致）。
type ToolHandler func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

// RegisterHandler 注册或覆盖工具处理器（开闭：产品工具不必改 CallNamedTool）。
// 核工具在首次 lookup 时由 ensureCoreHandlers 填入；之后 RegisterHandler 可覆盖同名。
func (h *Gateway) RegisterHandler(name string, fn ToolHandler) {
	if h == nil || fn == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	h.ensureCoreHandlers()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[name] = fn
}

// Handler 查找已注册处理器；无则 nil。
func (h *Gateway) Handler(name string) ToolHandler {
	if h == nil {
		return nil
	}
	h.ensureCoreHandlers()
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.handlers[strings.TrimSpace(name)]
}

// HasHandler 是否已注册。
func (h *Gateway) HasHandler(name string) bool {
	return h.Handler(name) != nil
}

func (h *Gateway) ensureCoreHandlers() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.coreRegistered {
		return
	}
	h.coreRegistered = true
	if h.handlers == nil {
		h.handlers = map[string]ToolHandler{}
	}
	for name, fn := range h.coreHandlerMap() {
		if _, exists := h.handlers[name]; !exists {
			h.handlers[name] = fn
		}
	}
}

// coreHandlerMap 核工具名 → 实现（CallNamedTool 与 MCP 共用同一批方法）。
func (h *Gateway) coreHandlerMap() map[string]ToolHandler {
	return map[string]ToolHandler{
		"list_tree":                     h.toolListTree,
		"search_session":                h.toolSearchSession,
		"recall_resource":               h.toolRecallResource,
		"read_file":                     h.toolReadFile,
		"grep":                          h.toolGrep,
		"write_file":                    h.toolWriteFile,
		"edit":                          h.toolEdit,
		"mkdir":                         h.toolMkdir,
		"create_file":                   h.toolCreateFile,
		"rename_path":                   h.toolRenamePath,
		"move_path":                     h.toolMovePath,
		"copy_path":                     h.toolCopyPath,
		"delete_path":                   h.toolDeletePath,
		"batch_delete_paths":            h.toolBatchDeletePaths,
		"batch_move_paths":              h.toolBatchMovePaths,
		"append_session_message":        h.toolAppendSession,
		"get_session_brief":             h.toolSessionBrief,
		"list_skills":                   h.toolListSkills,
		"get_skill":                     h.toolGetSkill,
		"create_project_skill":          h.toolCreateSkill,
		"append_project_skill_note":     h.toolAppendSkillNote,
		"set_lesson_status":             h.toolSetLessonStatus,
		"list_lessons":                  h.toolListLessons,
		"ack_lesson":                    h.toolAckLesson,
		"list_tasks":                    h.toolListTasks,
		"get_task_summary":              h.toolGetTaskSummary,
		"get_task_trace":                h.toolGetTaskTrace,
		"enqueue_agent_turn":            h.toolEnqueueAgentTurn,
		"enqueue_goal":                  h.toolEnqueueGoal,
		"enqueue_agent_turns_for_paths": h.toolEnqueueAgentTurnsForPaths,
		"list_queue":                    h.toolListQueue,
		"cancel_queue_task":             h.toolCancelQueueTask,
		"steer_queue_task":              h.toolSteerQueueTask,
		"set_queue_paused":              h.toolSetQueuePaused,
	}
}

func (h *Gateway) lookupHandler(name string) (ToolHandler, error) {
	h.ensureCoreHandlers()
	h.mu.Lock()
	fn := h.handlers[name]
	h.mu.Unlock()
	if fn == nil {
		return nil, fmt.Errorf("unknown tool %s", name)
	}
	return fn, nil
}
