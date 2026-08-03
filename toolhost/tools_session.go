package toolhost

import (
	"context"

	resource "ningharness/resource"
	session "ningharness/session"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *ToolHost) toolAppendSession(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("append_session_message", err)
	}
	role, err := req.RequireString("role")
	if err != nil {
		return toolErr("append_session_message", err)
	}
	content, err := req.RequireString("content")
	if err != nil {
		return toolErr("append_session_message", err)
	}
	sid := req.GetString("session_id", "")
	if _, err := h.sess.Append(root, h.pid(), sid, role, content, "", ""); err != nil {
		return toolErr("append_session_message", err)
	}
	if h.OnSessionChange != nil {
		h.OnSessionChange()
	}
	brief, _ := h.sess.ActiveBrief(root, h.pid())
	return mcp.NewToolResultText("ok\n" + brief), nil
}

func (h *ToolHost) toolSessionBrief(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("get_session_brief", err)
	}
	usersOnly := req.GetBool("users_only", false)
	limit := int(req.GetFloat("limit", 8))
	brief, err := h.sess.FormatThreadBrief(root, h.pid(), usersOnly, limit)
	if err != nil {
		return toolErr("get_session_brief", err)
	}
	return mcp.NewToolResultText(brief), nil
}

func (h *ToolHost) toolSearchSession(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("search_session", err)
	}
	query, err := req.RequireString("query")
	if err != nil {
		return toolErr("search_session", err)
	}
	limit := int(req.GetFloat("limit", 0))
	sessionID := req.GetString("session_id", "")
	text, err := h.sess.Search(root, h.pid(), session.SearchOptions{
		Query:     query,
		Limit:     limit,
		SessionID: sessionID,
	})
	if err != nil {
		return toolErr("search_session", err)
	}
	return mcp.NewToolResultText(text), nil
}

func (h *ToolHost) toolRecallResource(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	root, err := h.root()
	if err != nil {
		return toolErr("recall_resource", err)
	}
	resourceID := int64(req.GetFloat("resource_id", 0))
	text, err := resource.Search(root, resource.SearchOptions{
		ResourceID: resourceID,
		ToolCallID: req.GetString("tool_call_id", ""),
		RelPath:    req.GetString("rel_path", ""),
		Query:      req.GetString("query", ""),
		Phase:      req.GetString("phase", ""),
		Kind:       req.GetString("kind", ""),
		Limit:      int(req.GetFloat("limit", 0)),
	})
	if err != nil {
		return toolErr("recall_resource", err)
	}
	return mcp.NewToolResultText(text), nil
}
