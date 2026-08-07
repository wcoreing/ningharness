package toolgateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	deskqueue "ningharness/job"

	"github.com/mark3labs/mcp-go/mcp"
)

func (h *Gateway) CallNamedTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	if h == nil {
		return nil, fmt.Errorf("hub nil")
	}
	name = strings.TrimSpace(name)
	if err := h.checkToolInterceptor(name); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	args = FlattenArguments(args)
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	tr := h.TaskTrace()
	callID := ""
	if tr != nil {
		callID = traceCallIDFrom(ctx)
		if callID == "" {
			callID = newTraceCallID()
		}
		brief, _ := json.Marshal(argsBriefForTrace(name, args))
		_ = tr.ToolCall(callID, name, string(brief))
	}

	fn, err := h.lookupHandler(name)
	if err != nil {
		if tr != nil && callID != "" {
			_ = tr.ToolResult(callID, name, err.Error(), false)
		}
		return nil, err
	}
	res, err := fn(ctx, req)
	if err != nil {
		if tr != nil && callID != "" {
			_ = tr.ToolResult(callID, name, err.Error(), false)
		}
		return nil, err
	}
	res = h.applyToolOutputBound(callID, name, res)
	res = h.applyPendingSteer(res)
	if tr != nil && callID != "" {
		text := FormatToolResult(res)
		ok := res == nil || !res.IsError
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), "error:") {
			ok = false
		}
		_ = tr.ToolResult(callID, name, text, ok)
	}
	return res, nil
}

func (h *Gateway) applyPendingSteer(res *mcp.CallToolResult) *mcp.CallToolResult {
	if h == nil || res == nil {
		return res
	}
	jobID := h.TurnJobID()
	if jobID == "" || h.Queue == nil {
		return res
	}
	steer := h.Queue.TakeSteerPending(jobID)
	if strings.TrimSpace(steer) == "" {
		return res
	}
	block := deskqueue.FormatSteerBlock(steer)
	base := strings.TrimSpace(FormatToolResult(res))
	msg := base
	if msg != "" {
		msg += "\n\n"
	}
	msg += block
	out := *res
	out.Content = []mcp.Content{mcp.NewTextContent(msg)}
	return &out
}

func newTraceCallID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("tc-%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b[:]))
}

func argsBriefForTrace(name string, args map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range args {
		if k == "content" || k == "body" || k == "text" {
			if s, ok := v.(string); ok {
				out[k] = fmt.Sprintf("<%d chars>", len([]rune(s)))
				continue
			}
		}
		out[k] = v
	}
	if name == "" {
		return out
	}
	return out
}

func (h *Gateway) Invoke(ctx context.Context, name, argsJSON string) (string, error) {
	if h == nil {
		return "", fmt.Errorf("hub nil")
	}
	name = strings.TrimSpace(name)
	args := map[string]any{}
	if s := strings.TrimSpace(argsJSON); s != "" && s != "null" {
		s = UnwrapArgumentsJSON(s)
		if name == "write_file" {
			path, content, err := ParseWriteFile(s)
			if err != nil {
				return "", err
			}
			args["rel_path"] = path
			args["content"] = content
		} else if err := json.Unmarshal([]byte(s), &args); err != nil {
			return "", fmt.Errorf("args: %w", err)
		}
	}
	res, err := h.CallNamedTool(ctx, name, args)
	if err != nil {
		return "", err
	}
	return FormatToolResult(res), nil
}

func FormatToolResult(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range res.Content {
		switch t := c.(type) {
		case mcp.TextContent:
			b.WriteString(t.Text)
		default:
			raw, _ := json.Marshal(c)
			b.Write(raw)
		}
	}
	out := strings.TrimSpace(b.String())
	if res.IsError {
		if out == "" {
			out = "tool error"
		}
		return "error: " + out
	}
	return out
}
