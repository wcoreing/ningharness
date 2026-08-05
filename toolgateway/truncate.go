package toolgateway

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	maxToolVisibleRunes = 16000
	maxArchiveBytes     = 8*1024*1024 - 1
	relTruncDir         = ".ningharness/truncated"
)

// applyToolOutputBound 超长工具输出：归档全文（或有界头尾），返回模型可见窗口 + 路径 note。
// 所见=模型所见；归档失败不改变原输出。
func (h *Gateway) applyToolOutputBound(callID, toolName string, res *mcp.CallToolResult) *mcp.CallToolResult {
	if h == nil || res == nil || res.IsError {
		return res
	}
	full := FormatToolResult(res)
	if utf8.RuneCountInString(full) <= maxToolVisibleRunes {
		return res
	}
	root, err := h.root()
	if err != nil || strings.TrimSpace(root) == "" {
		return res
	}
	taskID, _, _ := h.TurnContext()
	if taskID == "" {
		taskID = "anon"
	}
	if callID == "" {
		callID = fmt.Sprintf("out-%d", time.Now().UnixMilli())
	}
	rel := filepath.ToSlash(filepath.Join(relTruncDir, sanitizeTracePath(taskID), sanitizeTracePath(callID)+".txt"))
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return res
	}
	body := []byte(full)
	noteExtra := ""
	if len(body) > maxArchiveBytes {
		head := body[:maxArchiveBytes/2]
		tail := body[len(body)-maxArchiveBytes/2:]
		body = append(append(head, []byte(fmt.Sprintf("\n\n[... %d bytes omitted ...]\n\n", len(full)-maxArchiveBytes))...), tail...)
		noteExtra = " archive_partial"
	}
	if err := os.WriteFile(abs, body, 0o600); err != nil {
		return res
	}
	visible := trimRunes(full, maxToolVisibleRunes)
	msg := fmt.Sprintf("%s\n\n[truncated: showing first %d chars; full output archived at %s — use read_file%s]",
		visible, maxToolVisibleRunes, rel, noteExtra)
	out := *res
	out.Content = []mcp.Content{mcp.NewTextContent(msg)}
	return &out
}

func sanitizeTracePath(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	if s == "" {
		return "x"
	}
	return s
}

func trimRunes(s string, n int) string {
	if n < 1 || utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
