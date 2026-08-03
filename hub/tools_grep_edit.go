package hub

import (
	"context"
	"path/filepath"
	"strings"

	"ningharness/workspace"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *Hub) toolGrep(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.contentRoot(); err != nil {
		return toolErr("grep", err)
	}
	pattern, err := req.RequireString("pattern")
	if err != nil {
		return toolErr("grep", err)
	}
	max := int(req.GetFloat("max_matches", 0))
	hits, err := h.ws.Grep(workspace.GrepOpts{
		Pattern:         pattern,
		Path:            strings.TrimSpace(req.GetString("path", "")),
		Glob:            strings.TrimSpace(req.GetString("glob", "")),
		CaseInsensitive: req.GetBool("case_insensitive", false),
		Regex:           req.GetBool("regex", false),
		MaxMatches:      max,
	})
	if err != nil {
		return toolErr("grep", err)
	}
	if max <= 0 {
		max = 40
	}
	return mcp.NewToolResultText(workspace.FormatGrepHits(pattern, hits, max)), nil
}

func (h *Hub) toolEdit(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.root(); err != nil {
		return toolErr("edit", err)
	}
	rel, err := req.RequireString("rel_path")
	if err != nil {
		return toolErr("edit", err)
	}
	old, err := req.RequireString("old_string")
	if err != nil {
		return toolErr("edit", err)
	}
	neu, err := req.RequireString("new_string")
	if err != nil {
		return toolErr("edit", err)
	}
	replaceAll := req.GetBool("replace_all", false)
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	body, err := h.readContentText(rel)
	if err != nil {
		return toolErr("edit", err)
	}
	out, n, err := workspace.ApplyEdit(body, old, neu, replaceAll)
	if err != nil {
		return toolErr("edit", err)
	}
	writeID := h.mcpWriteID("mcp-edit")
	if err := h.writeAgentFile(rel, out, writeID); err != nil {
		return toolErr("edit", err)
	}
	msg := workspace.FormatEditOK(rel, n, out, writeID)
	_ = ctx
	return mcp.NewToolResultText(msg), nil
}
