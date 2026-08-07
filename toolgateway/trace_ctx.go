package toolgateway

import (
	"context"
	"strings"
)

type ctxKey int

const traceCallIDKey ctxKey = 1

// WithTraceCallID 把 Eino/MCP 的 tool_call_id 带进 Gateway，写入 Task Trace 时与 resource 对齐。
func WithTraceCallID(ctx context.Context, id string) context.Context {
	id = strings.TrimSpace(id)
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, traceCallIDKey, id)
}

func traceCallIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(traceCallIDKey).(string)
	return strings.TrimSpace(s)
}
