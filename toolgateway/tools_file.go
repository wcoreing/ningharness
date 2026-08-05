package toolgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"ningharness/workspace"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *Gateway) toolListTree(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cr, err := h.contentRoot()
	if err != nil {
		return toolErr("list_tree", err)
	}
	pr, err := h.projectRoot()
	if err != nil {
		return toolErr("list_tree", err)
	}
	var listing workspace.TreeListing
	if filepath.Clean(cr) == filepath.Clean(pr) {
		listing, err = h.ws.ListTree()
	} else {
		listing, err = workspace.ListTreeAt(cr)
	}
	if err != nil {
		return toolErr("list_tree", err)
	}
	b, _ := json.MarshalIndent(listing, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (h *Gateway) toolReadFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.contentRoot(); err != nil {
		return toolErr("read_file", err)
	}
	rel, err := req.RequireString("rel_path")
	if err != nil {
		return toolErr("read_file", err)
	}
	body, err := h.readContentText(rel)
	if err != nil {
		return toolErr("read_file", err)
	}
	return mcp.NewToolResultText(body), nil
}

func (h *Gateway) toolWriteFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.root(); err != nil {
		return toolErr("write_file", err)
	}
	rel, err := req.RequireString("rel_path")
	if err != nil {
		return toolErr("write_file", err)
	}
	content, err := req.RequireString("content")
	if err != nil {
		return toolErr("write_file", err)
	}
	writeID := h.mcpWriteID("mcp-write")
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if err := h.writeAgentFile(rel, content, writeID); err != nil {
		return toolErr("write_file", err)
	}
	msg := formatWriteOK(rel, content, writeID)
	return mcp.NewToolResultText(msg), nil
}

func (h *Gateway) toolMkdir(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.contentRoot(); err != nil {
		return toolErr("mkdir", err)
	}
	rel, err := req.RequireString("rel_path")
	if err != nil {
		return toolErr("mkdir", err)
	}
	wid := h.mcpWriteID("mcp-mkdir")
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if err := h.mkdirContent(rel, wid); err != nil {
		return toolErr("mkdir", err)
	}
	h.notifyPathsChanged(wid, []string{rel}, nil)
	return mcp.NewToolResultText(fmt.Sprintf("mkdir %s (writeId=%s)", rel, wid)), nil
}

func (h *Gateway) toolCreateFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.contentRoot(); err != nil {
		return toolErr("create_file", err)
	}
	rel, err := req.RequireString("rel_path")
	if err != nil {
		return toolErr("create_file", err)
	}
	wid := h.mcpWriteID("mcp-create")
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if err := h.createFileContent(rel, wid); err != nil {
		return toolErr("create_file", err)
	}
	h.notifyPathsChanged(wid, []string{rel}, nil)
	return mcp.NewToolResultText(fmt.Sprintf("created %s (writeId=%s)", rel, wid)), nil
}

func (h *Gateway) toolRenamePath(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.root(); err != nil {
		return toolErr("rename_path", err)
	}
	rel, err := req.RequireString("rel_path")
	if err != nil {
		return toolErr("rename_path", err)
	}
	name, err := req.RequireString("new_name")
	if err != nil {
		return toolErr("rename_path", err)
	}
	wid := h.mcpWriteID("mcp-rename")
	res, err := h.ws.Rename(rel, name, wid)
	if err != nil {
		return toolErr("rename_path", err)
	}
	h.remapFileGitAfterMove(res)
	h.notifyPathsChanged(wid, mutationRevealPaths(res), nil)
	return mutationJSON(res), nil
}

func (h *Gateway) toolMovePath(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.root(); err != nil {
		return toolErr("move_path", err)
	}
	from, err := req.RequireString("from_rel")
	if err != nil {
		return toolErr("move_path", err)
	}
	to, err := req.RequireString("to_rel")
	if err != nil {
		return toolErr("move_path", err)
	}
	wid := h.mcpWriteID("mcp-move")
	res, err := h.ws.Move(from, to, wid)
	if err != nil {
		return toolErr("move_path", err)
	}
	h.remapFileGitAfterMove(res)
	h.notifyPathsChanged(wid, mutationRevealPaths(res), nil)
	return mutationJSON(res), nil
}

func (h *Gateway) toolCopyPath(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.root(); err != nil {
		return toolErr("copy_path", err)
	}
	from, err := req.RequireString("from_rel")
	if err != nil {
		return toolErr("copy_path", err)
	}
	to, err := req.RequireString("to_rel")
	if err != nil {
		return toolErr("copy_path", err)
	}
	wid := h.mcpWriteID("mcp-copy")
	res, err := h.ws.Copy(from, to, wid)
	if err != nil {
		return toolErr("copy_path", err)
	}
	h.notifyPathsChanged(wid, mutationRevealPaths(res), nil)
	return mutationJSON(res), nil
}

func (h *Gateway) toolDeletePath(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.root(); err != nil {
		return toolErr("delete_path", err)
	}
	rel, err := req.RequireString("rel_path")
	if err != nil {
		return toolErr("delete_path", err)
	}
	wid := h.mcpWriteID("mcp-delete")
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if err := h.ws.Delete(rel, wid); err != nil {
		return toolErr("delete_path", err)
	}
	h.notifyPathsChanged(wid, []string{rel}, nil)
	return mcp.NewToolResultText(fmt.Sprintf("deleted %s (writeId=%s)", rel, wid)), nil
}

func (h *Gateway) toolBatchDeletePaths(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.root(); err != nil {
		return toolErr("batch_delete_paths", err)
	}
	rels, err := req.RequireStringSlice("rel_paths")
	if err != nil {
		return toolErr("batch_delete_paths", err)
	}
	wid := h.mcpWriteID("mcp-batch-delete")
	res := h.ws.BatchDelete(rels, wid)
	h.notifyPathsChanged(wid, mutationRevealPaths(res), nil)
	return mutationJSON(res), nil
}

func (h *Gateway) toolBatchMovePaths(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := h.root(); err != nil {
		return toolErr("batch_move_paths", err)
	}
	rels, err := req.RequireStringSlice("rel_paths")
	if err != nil {
		return toolErr("batch_move_paths", err)
	}
	dest := req.GetString("dest_dir", "")
	wid := h.mcpWriteID("mcp-batch-move")
	res := h.ws.BatchMove(rels, dest, wid)
	h.remapFileGitAfterMove(res)
	h.notifyPathsChanged(wid, mutationRevealPaths(res), nil)
	return mutationJSON(res), nil
}
