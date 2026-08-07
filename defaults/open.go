// Package defaults 可选装配层：粘合 Harness + ToolGateway + MCP + Guest + Lifecycle Host。
// 实现 lifecycle.Host（含 RunState→ToolGateway 投影同步）；不定义生命周期步骤表本身。
// 仅 import defaults 并 Open 才启用；可关掉 MCP / 换掉 Guest。
package defaults

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ningharness"
	"ningharness/guest"
	"ningharness/guest/eino"
	"ningharness/job"
	"ningharness/lifecycle"
	"ningharness/memory"
	"ningharness/skill"
	"ningharness/toolgateway"
	"ningharness/workspace"

	"github.com/mark3labs/mcp-go/server"
)

// Opts 默认装配选项。
type Opts struct {
	ningharness.Opts

	// MCPAddr MCP HTTP 监听；空=默认 127.0.0.1:51020；"off"/"-"=不启动。
	MCPAddr string
	// MCPServerName 覆盖 MCP serverInfo.name；空=ningharness。
	MCPServerName string
	// MCPVersion 覆盖 MCP serverInfo.version。
	MCPVersion string
	// MCPInstructions 覆盖 MCP instructions。
	MCPInstructions string
	// WithoutEino 为 true 时不创建默认 Eino Guest（可稍后 SetGuest）。
	WithoutEino bool
	// WithoutMemory 为 true 时不装默认 Lesson Memory（可稍后 SetMemory）。
	WithoutMemory bool
	// WithoutSkill 为 true 时不装默认 Disk Skill（可稍后 SetSkill）。
	WithoutSkill bool
	// Eino 覆盖默认 Guest 配置（API Key 等）。
	Eino eino.Opts
	// PrepareGateway 在 ToolGateway 建好、MCP 启动前调用（可 RegisterHandler）。
	PrepareGateway func(th *toolgateway.Gateway)
	// ExtraTools 传给 MCP HTTP（RegisterCoreTools 之后 AddTool）。
	ExtraTools func(s *server.MCPServer, h *toolgateway.Gateway)
}

// Runtime 带默认 ToolGateway / MCP / Guest / Memory / Skill / Lifecycle 的运行时。
type Runtime struct {
	*ningharness.Harness
	ToolGateway *toolgateway.Gateway
	MCP         *toolgateway.HTTPService
	Guest       guest.Guest
	// Memory assemble 时贡献前馈；nil=不补充。
	Memory memory.Memory
	// Skill 方法包插槽；nil=assemble 不做路径匹配。
	Skill skill.Slot
	// Lifecycle 默认 Task/Chat 生命周期；可 Clone 后改步骤或 SetLifecycle 替换。
	Lifecycle *lifecycle.Lifecycle
	// ExtraTools 客户端工具注册（嵌入 mux 时 NewMCPHTTPHandler 复用）。
	ExtraTools func(s *server.MCPServer, h *toolgateway.Gateway)
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

	rt := &Runtime{Harness: h, ToolGateway: th, ExtraTools: opts.ExtraTools}
	rt.Lifecycle = lifecycle.NewDefault(rt)
	if !opts.WithoutMemory {
		rt.Memory = memory.NewLesson()
	}
	if !opts.WithoutSkill {
		rt.Skill = skill.NewDisk()
	}

	if opts.PrepareGateway != nil {
		opts.PrepareGateway(th)
	}

	addr := strings.TrimSpace(opts.MCPAddr)
	if !isOff(addr) {
		if addr == "" {
			addr = toolgateway.DefaultHTTPAddr
		}
		serverName := strings.TrimSpace(opts.MCPServerName)
		if serverName == "" {
			serverName = "ningharness"
		}
		instructions := strings.TrimSpace(opts.MCPInstructions)
		if instructions == "" {
			instructions = "ningharness core tools (files, session, skill, lesson, task, queue)"
		}
		mcp, err := toolgateway.StartHTTP(toolgateway.HTTPConfig{
			Addr:         addr,
			ServerName:   serverName,
			Version:      opts.MCPVersion,
			Instructions: instructions,
			HealthName:   serverName,
			ExtraTools:   opts.ExtraTools,
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

// SetMemory 替换或清空 Memory（nil=assemble 不追加记忆前馈）。
func (rt *Runtime) SetMemory(m memory.Memory) {
	if rt == nil {
		return
	}
	rt.Memory = m
}

// SetSkill 替换或清空 Skill Slot（nil=assemble 不做路径匹配补 skill id）。
func (rt *Runtime) SetSkill(s skill.Slot) {
	if rt == nil {
		return
	}
	rt.Skill = s
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

// NewMCPHTTPHandler 构建可嵌入产品 mux 的 Streamable MCP handler（含 ExtraTools）。
func (rt *Runtime) NewMCPHTTPHandler(serverName, version, instructions string) http.Handler {
	if rt == nil || rt.ToolGateway == nil {
		return nil
	}
	return toolgateway.NewMCPHTTPHandler(rt.ToolGateway, toolgateway.HTTPConfig{
		ServerName:   serverName,
		Version:      version,
		Instructions: instructions,
		ExtraTools:   rt.ExtraTools,
	})
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
