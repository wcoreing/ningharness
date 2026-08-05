package toolgateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"ningharness/workspace"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *Gateway) Pid() string { return h.pid() }

func (h *Gateway) MCPWriteID(prefix string) string { return h.mcpWriteID(prefix) }

func (h *Gateway) NotifyPathsChanged(writeID string, paths []string, wordCounts map[string]int) {
	h.notifyPathsChanged(writeID, paths, wordCounts)
}

func (h *Gateway) WriteAgentFile(rel, content, writeID string) error {
	return h.writeAgentFile(rel, content, writeID)
}

func ToolErr(name string, err error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(fmt.Sprintf("%s: %v", name, err)), nil
}

func toolErr(name string, err error) (*mcp.CallToolResult, error) {
	return ToolErr(name, err)
}

func (h *Gateway) mcpWriteID(prefix string) string {
	p := strings.TrimSpace(prefix)
	if p == "" {
		p = "mcp"
	}
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%d-%s", p, time.Now().UnixMilli(), hex.EncodeToString(b[:]))
}

func (h *Gateway) writeAgentFile(rel, content, writeID string) error {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if h.OnWriteWorktree != nil {
		return h.OnWriteWorktree(rel, content, writeID)
	}
	return h.ws.WriteText(rel, content, writeID)
}

func (h *Gateway) notifyPathsChanged(writeID string, paths []string, wordCounts map[string]int) {
	if h == nil || h.OnPathsChanged == nil || len(paths) == 0 {
		return
	}
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p != "" {
			clean = append(clean, p)
		}
	}
	if len(clean) == 0 {
		return
	}
	h.OnPathsChanged(strings.TrimSpace(writeID), clean, wordCounts)
}

func MutationRevealPaths(res workspace.MutationResult) []string {
	return mutationRevealPaths(res)
}

func mutationRevealPaths(res workspace.MutationResult) []string {
	if len(res.MovedTo) > 0 {
		out := make([]string, 0, len(res.MovedTo))
		for _, to := range res.MovedTo {
			to = filepath.ToSlash(strings.TrimSpace(to))
			if to != "" {
				out = append(out, to)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	out := make([]string, 0, len(res.OK))
	for _, p := range res.OK {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func MutationJSON(res workspace.MutationResult) *mcp.CallToolResult {
	return mutationJSON(res)
}

func mutationJSON(res workspace.MutationResult) *mcp.CallToolResult {
	b, _ := json.MarshalIndent(res, "", "  ")
	return mcp.NewToolResultText(string(b))
}

func (h *Gateway) remapFileGitAfterMove(res workspace.MutationResult) {
	if h == nil || h.OnPathsMoved == nil || len(res.MovedTo) == 0 {
		return
	}
	h.OnPathsMoved(res.MovedTo)
}
