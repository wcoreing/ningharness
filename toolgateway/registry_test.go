package toolgateway

import (
	"context"
	"strings"
	"testing"

	"ningharness/workspace"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestRegistryLookupCoreAndCustom(t *testing.T) {
	root := t.TempDir()
	ws := workspace.New()
	if _, err := ws.Open(root); err != nil {
		t.Fatal(err)
	}
	h := New(ws, nil)
	if !h.HasHandler("list_tree") {
		t.Fatal("core list_tree missing")
	}
	if h.HasHandler("nope") {
		t.Fatal("unknown should be absent")
	}
	_, err := h.CallNamedTool(context.Background(), "nope", nil)
	if err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("err=%v", err)
	}

	h.RegisterHandler("echo_test", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok-echo"), nil
	})
	res, err := h.CallNamedTool(context.Background(), "echo_test", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if FormatToolResult(res) != "ok-echo" {
		t.Fatalf("got %q", FormatToolResult(res))
	}
}

func TestRegistryOverrideCore(t *testing.T) {
	h := New(workspace.New(), nil)
	h.RegisterHandler("list_tree", func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("stub-tree"), nil
	})
	res, err := h.CallNamedTool(context.Background(), "list_tree", nil)
	if err != nil {
		t.Fatal(err)
	}
	if FormatToolResult(res) != "stub-tree" {
		t.Fatalf("got %q", FormatToolResult(res))
	}
}
