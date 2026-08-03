// Package defaults 可选系统默认装配：ToolHost 核工具 + MCP HTTP + Eino Guest。
// 仅 Open 根包时不含这些；import defaults 并 Open 才启用。可关掉 MCP / 换掉 Guest。
package defaults

import (
	"context"
	"fmt"
	"strings"

	"ningharness"
	"ningharness/guest"
	"ningharness/guest/eino"
	"ningharness/job"
	"ningharness/toolhost"
	"ningharness/workspace"
)

// Opts 默认装配选项。
type Opts struct {
	ningharness.Opts

	// MCPAddr MCP HTTP 监听；空=默认 127.0.0.1:51020；"off"/"-"=不启动。
	MCPAddr string
	// WithoutEino 为 true 时不创建默认 Eino Guest（可稍后 SetGuest）。
	WithoutEino bool
	// Eino 覆盖默认 Guest 配置（API Key 等）。
	Eino eino.Opts
}

// Runtime 带默认 ToolHost / MCP / Guest 的运行时。
type Runtime struct {
	*ningharness.Harness
	ToolHost *toolhost.ToolHost
	MCP      *toolhost.HTTPService
	Guest    guest.Guest
}

// Open 打开地基并装配默认 ToolHost、核工具 MCP、可选 Eino Guest。
func Open(opts Opts) (*Runtime, error) {
	h, err := ningharness.Open(opts.Opts)
	if err != nil {
		return nil, err
	}
	ws := workspace.New()
	if root := h.Root(); root != "" {
		if _, err := ws.Open(root); err != nil {
			_ = h.Close()
			return nil, err
		}
	}
	th := toolhost.New(ws, h.Session)
	th.Queue = h.Job
	if th.OnWriteWorktree == nil {
		th.OnWriteWorktree = func(rel, content, writeID string) error {
			return ws.WriteText(rel, content, writeID)
		}
	}

	rt := &Runtime{Harness: h, ToolHost: th}

	addr := strings.TrimSpace(opts.MCPAddr)
	if !isOff(addr) {
		if addr == "" {
			addr = toolhost.DefaultHTTPAddr
		}
		mcp, err := toolhost.StartHTTP(toolhost.HTTPConfig{
			Addr:         addr,
			ServerName:   "ningharness",
			Instructions: "ningharness core tools (files, session, skill, lesson, task, queue)",
		}, th)
		if err != nil {
			_ = h.Close()
			return nil, fmt.Errorf("defaults mcp: %w", err)
		}
		rt.MCP = mcp
	}

	if !opts.WithoutEino {
		g, err := eino.New(th, opts.Eino)
		if err != nil {
			_ = rt.Close()
			return nil, err
		}
		rt.Guest = g
	}

	// Job 默认 Executor：有 Guest 时用 Chat 跑一轮（单步 Job）。
	if h.Job != nil && rt.Guest != nil {
		g := rt.Guest
		h.Job.SetExecutor(func(ctx context.Context, j job.Job) (string, error) {
			prompt := strings.TrimSpace(j.WirePrompt)
			if prompt == "" {
				prompt = strings.TrimSpace(j.Prompt)
			}
			reply, err := g.Chat(ctx, prompt)
			if err != nil {
				return "", err
			}
			taskID := "once:" + j.ID
			_, _ = h.Session.Append(h.Root(), "", j.SessionKey, "assistant", reply, taskID, "")
			return taskID, nil
		})
	}

	return rt, nil
}

// SetGuest 替换或清空 Guest（nil=禁用 Chat）。
func (rt *Runtime) SetGuest(g guest.Guest) {
	if rt == nil {
		return
	}
	rt.Guest = g
}

// Chat 用当前 Guest 发一句话；无 Guest 时报错。
func (rt *Runtime) Chat(ctx context.Context, message string) (string, error) {
	if rt == nil || rt.Guest == nil {
		return "", fmt.Errorf("defaults: no Guest (Open with WithoutEino=false or SetGuest)")
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "", fmt.Errorf("defaults: empty message")
	}
	root := rt.Root()
	sess := "main"
	if _, err := rt.Session.Append(root, "", sess, "user", msg, "", ""); err != nil {
		// 无项目时 Append 可能失败；仍尝试 Chat
		_ = err
	}
	reply, err := rt.Guest.Chat(ctx, msg)
	if err != nil {
		return "", err
	}
	_, _ = rt.Session.Append(root, "", sess, "assistant", reply, "", "")
	return reply, nil
}

// MCPURL 当前 MCP 端点；未启动则空。
func (rt *Runtime) MCPURL() string {
	if rt == nil || rt.MCP == nil {
		return ""
	}
	return rt.MCP.EndpointURL()
}

// Close 停 MCP 并关闭 Harness。
func (rt *Runtime) Close() error {
	if rt == nil {
		return nil
	}
	if rt.MCP != nil {
		_ = rt.MCP.Stop(context.Background())
		rt.MCP = nil
	}
	if rt.Harness != nil {
		return rt.Harness.Close()
	}
	return nil
}

func isOff(addr string) bool {
	switch strings.ToLower(strings.TrimSpace(addr)) {
	case "off", "-", "none", "false":
		return true
	default:
		return false
	}
}
