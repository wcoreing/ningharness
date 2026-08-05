package toolgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agenttask "ningharness/task"
	"ningharness/trace"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *Gateway) toolListTasks(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("list_tasks", err)
	}
	limit := 20
	if n := req.GetFloat("limit", 0); n > 0 {
		limit = int(n)
	}
	includeReflect := false
	if args, ok := req.Params.Arguments.(map[string]any); ok {
		if v, exists := args["include_reflect"]; exists {
			switch t := v.(type) {
			case bool:
				includeReflect = t
			case string:
				includeReflect = strings.EqualFold(t, "true") || t == "1"
			}
		}
	}
	list, err := agenttask.List(root, limit, !includeReflect)
	if err != nil {
		return toolErr("list_tasks", err)
	}
	b, _ := json.MarshalIndent(list, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (h *Gateway) toolGetTaskSummary(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("get_task_summary", err)
	}
	id := req.GetString("task_id", "")
	rec, err := agenttask.Get(root, id)
	if err != nil {
		return toolErr("get_task_summary", err)
	}
	return mcp.NewToolResultText(agenttask.FormatSummary(rec)), nil
}

func (h *Gateway) toolGetTaskTrace(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("get_task_trace", err)
	}
	id := strings.TrimSpace(req.GetString("task_id", ""))
	if id == "" {
		return toolErr("get_task_trace", fmt.Errorf("task_id required"))
	}
	path, err := trace.FindByTaskID(root, id)
	if err != nil {
		return toolErr("get_task_trace", err)
	}
	st, evs, err := trace.InspectFile(path)
	if err != nil {
		return toolErr("get_task_trace", err)
	}
	limit := 80
	if n := req.GetFloat("limit", 0); n > 0 {
		limit = int(n)
	}
	if limit > 500 {
		limit = 500
	}
	if len(evs) > limit {
		evs = evs[len(evs)-limit:]
	}
	payload := map[string]any{
		"path":           path,
		"complete":       st.Complete,
		"ended_status":   st.EndedStatus,
		"unpaired_calls": st.UnpairedCalls,
		"event_count":    st.EventCount,
		"events":         evs,
		"note":           "恢复契约：仅认已配对 tool_call/tool_result；无 task_end 或有 unpaired 则未收口。正文仍以文件+resource 为准。",
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}
