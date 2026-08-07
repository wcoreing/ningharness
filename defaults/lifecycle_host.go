package defaults

import (
	"context"
	"fmt"
	"strings"

	"ningharness/guest"
	"ningharness/lifecycle"
	"ningharness/memory"
	"ningharness/skill"
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

// AssembleContext 宿主侧本轮输入面：Skill 匹配 → Memory 追加前馈 → 落 user。
// 客户端可先写 st.Feedforward；Gate 仍可加厚。
// Values：memory.SkillIDsValueKey、skill.PathsValueKey。
func (rt *Runtime) AssembleContext(ctx context.Context, st *lifecycle.RunState) error {
	if rt == nil || st == nil {
		return fmt.Errorf("defaults: nil runtime/state")
	}
	root := st.Root
	if root == "" {
		root = rt.Root()
		st.Root = root
	}
	skillIDs := memory.SkillIDsFromValues(st.Values)
	if len(skillIDs) == 0 && rt.Skill != nil {
		if paths := skill.PathsFromValues(st.Values); len(paths) > 0 {
			skillIDs = skill.IDs(rt.Skill.Match(root, paths))
			if len(skillIDs) > 0 {
				st.Set(memory.SkillIDsValueKey, skillIDs)
			}
		}
	}
	ff := strings.TrimSpace(st.Feedforward)
	if rt.Memory != nil {
		patch, err := rt.Memory.Assemble(ctx, memory.AssembleInput{
			Root:                root,
			SessionKey:          st.SessionKey,
			TaskID:              st.TaskID,
			Prompt:              st.Prompt,
			ExistingFeedforward: ff,
			SkillIDs:            skillIDs,
		})
		if err != nil {
			return err
		}
		ff = memory.MergeFeedforward(ff, patch)
		st.Feedforward = ff
	}
	if st.SkipUserAppend && ff == "" {
		return nil
	}
	prompt := strings.TrimSpace(st.Prompt)
	if prompt == "" && ff == "" {
		return nil
	}
	// 仅有前馈、无 prompt 时仍落一行 user，保证 history.feedforward 可检索。
	if prompt == "" {
		prompt = "(context)"
	}
	_, err := rt.Session.Append(root, "", st.SessionKey, "user", prompt, st.TaskID, ff)
	return err
}

// RunGuest 调用 Guest.Run（前馈经 guest.Wire 进模）。
func (rt *Runtime) RunGuest(ctx context.Context, st *lifecycle.RunState) error {
	if rt == nil || st == nil {
		return fmt.Errorf("defaults: nil runtime/state")
	}
	if rt.Guest == nil {
		return fmt.Errorf("defaults: no Guest")
	}
	reply, err := rt.Guest.Run(ctx, guest.Input{
		Message:     st.Prompt,
		Feedforward: st.Feedforward,
	})
	if err != nil {
		return err
	}
	st.Reply = reply
	return nil
}

// PersistTurn 落 assistant 气泡；若 Memory 实现 Ingester 则随后沉淀。
func (rt *Runtime) PersistTurn(ctx context.Context, st *lifecycle.RunState) error {
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
	if _, err := rt.Session.Append(root, "", st.SessionKey, "assistant", reply, st.TaskID, ""); err != nil {
		return err
	}
	if ing, ok := rt.Memory.(memory.Ingester); ok {
		return ing.Ingest(ctx, memory.IngestInput{
			Root:        root,
			SessionKey:  st.SessionKey,
			TaskID:      st.TaskID,
			Prompt:      st.Prompt,
			Reply:       reply,
			Feedforward: st.Feedforward,
			SkillIDs:    memory.SkillIDsFromValues(st.Values),
		})
	}
	return nil
}

// EndTask 生命周期钩子点（供 Bus after）；投影/Trace 收尾只在 OnExit→FinishTurn。
func (rt *Runtime) EndTask(ctx context.Context, st *lifecycle.RunState) error {
	_ = ctx
	_ = st
	_ = rt
	return nil
}

var _ lifecycle.Host = (*Runtime)(nil)
