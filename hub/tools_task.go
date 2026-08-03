package hub

import (
	"context"
	"encoding/json"
	"strings"

	agenttask "ningharness/task"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *Hub) toolListTasks(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func (h *Hub) toolGetTaskSummary(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
