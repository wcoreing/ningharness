package toolgateway

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// boundHandler MCP schema 回调走 lookupHandler，使 RegisterHandler 覆盖对 MCP 生效。
func (h *Gateway) boundHandler(name string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if h == nil {
			return mcp.NewToolResultError("gateway nil"), nil
		}
		fn, err := h.lookupHandler(name)
		if err != nil {
			return toolErr(name, err)
		}
		return fn(ctx, req)
	}
}
