// Package defaults 可选装配层：粘合 Harness + ToolGateway + MCP + Guest + Lifecycle Host。
// 实现 lifecycle.Host（含 RunState→ToolGateway 投影同步）；不定义生命周期步骤表本身。
// 仅 import defaults 并 Open 才启用；可关掉 MCP / 换掉 Guest。
package defaults

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ningharness"
	"ningharness/guest"
	"ningharness/guest/eino"
	"ningharness/job"
	"ningharness/lifecycle"
	"ningharness/toolgateway"
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

// Runtime 带默认 ToolGateway / MCP / Guest / Lifecycle 的运行时。
type Runtime struct {
	*ningharness.Harness
	ToolGateway *toolgateway.Gateway
	MCP         *toolgateway.HTTPService
	Guest       guest.Guest
	// Lifecycle 默认 Task/Chat 生命周期；可 Clone 后改步骤或 SetLifecycle 替换。
	Lifecycle *lifecycle.Lifecycle
}

// Open 打开地基并装配默认 ToolGateway、核工具 MCP、可选 Eino Guest、默认生命周期。
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
	th := toolgateway.New(ws, h.Session)
	th.Queue = h.Job
	if th.OnWriteWorktree == nil {
		th.OnWriteWorktree = func(rel, content, writeID string) error {
			return ws.WriteText(rel, content, writeID)
		}
	}

	rt := &Runtime{Harness: h, ToolGateway: th}
	rt.Lifecycle = lifecycle.NewDefault(rt)

	addr := strings.TrimSpace(opts.MCPAddr)
	if !isOff(addr) {
		if addr == "" {
			addr = toolgateway.DefaultHTTPAddr
		}
		mcp, err := toolgateway.StartHTTP(toolgateway.HTTPConfig{
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

	rt.bindJobExecutor()
	return rt, nil
}

func (rt *Runtime) bindJobExecutor() {
	if rt == nil || rt.Job == nil || rt.Guest == nil {
		return
	}
	rt.Job.SetExecutor(func(ctx context.Context, j job.Job) (string, error) {
		prompt := strings.TrimSpace(j.WirePrompt)
		if prompt == "" {
			prompt = strings.TrimSpace(j.Prompt)
		}
		taskID := taskIDForJob(j)
		sess := strings.TrimSpace(j.SessionKey)
		st := &lifecycle.RunState{
			Root:           rt.Root(),
			SessionKey:     sess,
			TaskID:         taskID,
			JobID:          j.ID,
			Prompt:         prompt,
			Feedforward:    strings.TrimSpace(j.FeedExtra),
			SkipUserAppend: true, // 无 FeedExtra 时不落 user；有则 assemble 仍落（带前馈）
		}
		if err := rt.runLifecycle(ctx, st); err != nil {
			return "", err
		}
		return taskID, nil
	})
}

func (rt *Runtime) runLifecycle(ctx context.Context, st *lifecycle.RunState) error {
	if rt == nil {
		return fmt.Errorf("defaults: nil runtime")
	}
	lc := rt.Lifecycle
	if lc == nil {
		lc = lifecycle.NewDefault(rt)
	}
	ctx = lifecycle.WithRunState(ctx, st)
	return (lifecycle.Runner{}).Run(ctx, lc, st)
}

// SetGuest 替换或清空 Guest（nil=禁用 Chat）；有 Guest 时重绑 Job Executor。
func (rt *Runtime) SetGuest(g guest.Guest) {
	if rt == nil {
		return
	}
	rt.Guest = g
	rt.bindJobExecutor()
}

// SetLifecycle 替换生命周期（nil 则下次 run 回退 NewDefault(rt)）。
func (rt *Runtime) SetLifecycle(lc *lifecycle.Lifecycle) {
	if rt == nil {
		return
	}
	rt.Lifecycle = lc
}

// Chat 用当前 Guest 经默认生命周期发一句话；无 Guest 时报错。
func (rt *Runtime) Chat(ctx context.Context, message string) (string, error) {
	if rt == nil || rt.Guest == nil {
		return "", fmt.Errorf("defaults: no Guest (Open with WithoutEino=false or SetGuest)")
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "", fmt.Errorf("defaults: empty message")
	}
	st := &lifecycle.RunState{
		Root:       rt.Root(),
		SessionKey: "main",
		TaskID:     fmt.Sprintf("chat:%d", time.Now().UnixMilli()),
		Prompt:     msg,
	}
	if err := rt.runLifecycle(ctx, st); err != nil {
		return "", err
	}
	return st.Reply, nil
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
