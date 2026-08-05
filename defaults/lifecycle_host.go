package defaults

import (
	"context"
	"fmt"
	"strings"

	"ningharness/lifecycle"
)

// BeginTask 开 Trace，ProjectTurn 投影；OnExit→FinishTurn 为唯一收尾路径。
func (rt *Runtime) BeginTask(ctx context.Context, st *lifecycle.RunState) error {
	_ = ctx
	if rt == nil || st == nil {
		return fmt.Errorf("defaults: nil runtime/state")
	}
	th := rt.ToolGateway
	if th == nil {
		return nil
	}
	root := strings.TrimSpace(st.Root)
	if root == "" {
		root = rt.Root()
		st.Root = root
	}
	th.ArmTaskTrace(root, st.TaskID, st.SessionKey, st.JobID)
	th.ProjectTurn(st.TaskID, st.SessionKey, "", st.JobID)
	st.OnExit(func(err error) {
		status := "ok"
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
			status = "error"
			if strings.Contains(errMsg, "context canceled") || strings.Contains(errMsg, "context cancelled") {
				status = "cancelled"
			}
		}
		th.FinishTurn(status, errMsg)
	})
	return nil
}

// AssembleContext 宿主侧本轮输入面：落 user 气泡，并带上 Feedforward（若有）。
// 路径索引 / skill 目录等仍可由 Gate 加厚。
func (rt *Runtime) AssembleContext(ctx context.Context, st *lifecycle.RunState) error {
	_ = ctx
	if rt == nil || st == nil {
		return fmt.Errorf("defaults: nil runtime/state")
	}
	ff := strings.TrimSpace(st.Feedforward)
	if st.SkipUserAppend && ff == "" {
		return nil
	}
	prompt := strings.TrimSpace(st.Prompt)
	if prompt == "" && ff == "" {
		return nil
	}
	root := st.Root
	if root == "" {
		root = rt.Root()
	}
	// 仅有前馈、无 prompt 时仍落一行 user，保证 history.feedforward 可检索。
	if prompt == "" {
		prompt = "(context)"
	}
	_, err := rt.Session.Append(root, "", st.SessionKey, "user", prompt, st.TaskID, ff)
	return err
}

// RunGuest 调用 Guest.Chat；工具环在 Guest/Gateway 内。
func (rt *Runtime) RunGuest(ctx context.Context, st *lifecycle.RunState) error {
	if rt == nil || st == nil {
		return fmt.Errorf("defaults: nil runtime/state")
	}
	if rt.Guest == nil {
		return fmt.Errorf("defaults: no Guest")
	}
	reply, err := rt.Guest.Chat(ctx, st.Prompt)
	if err != nil {
		return err
	}
	st.Reply = reply
	return nil
}

// PersistTurn 落 assistant 气泡。
func (rt *Runtime) PersistTurn(ctx context.Context, st *lifecycle.RunState) error {
	_ = ctx
	if rt == nil || st == nil {
		return fmt.Errorf("defaults: nil runtime/state")
	}
	reply := strings.TrimSpace(st.Reply)
	if reply == "" {
		return nil
	}
	root := st.Root
	if root == "" {
		root = rt.Root()
	}
	_, err := rt.Session.Append(root, "", st.SessionKey, "assistant", reply, st.TaskID, "")
	return err
}

// EndTask 生命周期钩子点（供 Bus after）；投影/Trace 收尾只在 OnExit→FinishTurn。
func (rt *Runtime) EndTask(ctx context.Context, st *lifecycle.RunState) error {
	_ = ctx
	_ = st
	_ = rt
	return nil
}

var _ lifecycle.Host = (*Runtime)(nil)
