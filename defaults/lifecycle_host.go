package defaults

import (
	"context"
	"fmt"
	"strings"

	"github.com/wcoreing/ningharness/guest"
	"github.com/wcoreing/ningharness/learning"
	"github.com/wcoreing/ningharness/lifecycle"
	"github.com/wcoreing/ningharness/memory"
	"github.com/wcoreing/ningharness/skill"
)

// BeginTask 开 Trace，ProjectTurn 投影；按 purpose 武装工具层；OnExit→FinishTurn 为唯一收尾路径。
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
	if p, ok := st.Get("purpose"); ok {
		if ps, _ := p.(string); strings.TrimSpace(ps) != "" {
			th.ArmPurpose(ps)
		}
	}
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
	lessonIDs := skill.FilterLessonInjectIDs(skillIDs)
	callerFF := strings.TrimSpace(st.Feedforward)
	ff := callerFF
	if rt.Memory != nil {
		patch, err := rt.Memory.Assemble(ctx, memory.AssembleInput{
			Root:                root,
			SessionKey:          st.SessionKey,
			TaskID:              st.TaskID,
			Prompt:              st.Prompt,
			ExistingFeedforward: ff,
			SkillIDs:            lessonIDs,
		})
		if err != nil {
			return err
		}
		ff = memory.MergeFeedforward(ff, patch)
	}
	if hint := skill.MountBrief(root, skillIDs); hint != "" {
		ff = memory.MergeFeedforward(ff, hint)
	}
	st.Feedforward = ff
	prompt := strings.TrimSpace(st.Prompt)
	// SkipUserAppend：确认续跑 / Job 无正文时不落气泡；仅调用方显式带了 Feedforward 且无 prompt 时落 "(context)" 占位便于检索。
	if st.SkipUserAppend {
		if prompt == "" && callerFF == "" {
			return nil
		}
		if prompt == "" {
			prompt = "(context)"
		}
		_, err := rt.Session.Append(root, "", st.SessionKey, "user", prompt, st.TaskID, ff)
		return err
	}
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

// RunGuest 思考步：按 Values[think] 走本地 Guest 或外置 External（同一步骤，同一 Lifecycle）。
func (rt *Runtime) RunGuest(ctx context.Context, st *lifecycle.RunState) error {
	if rt == nil || st == nil {
		return fmt.Errorf("defaults: nil runtime/state")
	}
	if guest.ThinkMode(st.Values) == guest.ThinkExternal {
		return rt.runExternal(ctx, st)
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

// runExternal 外置大脑；AssembleContext 已落 user，ExternalTurn.SkipUserAppend=true。
func (rt *Runtime) runExternal(ctx context.Context, st *lifecycle.RunState) error {
	if rt.External == nil {
		return fmt.Errorf("defaults: no External（SetExternal 后再 think=external）")
	}
	if rt.Tools == nil {
		return fmt.Errorf("defaults: no Tools（SetTools 后再 think=external）")
	}
	emit := rt.OnExternalEvent
	if emit == nil {
		emit = func(guest.Event) {}
	}
	in := guest.ExternalTurnFromState(guest.StateView{
		Root: st.Root, SessionKey: st.SessionKey, TaskID: st.TaskID,
		Prompt: st.Prompt, Feedforward: st.Feedforward,
		SkipUserAppend: st.SkipUserAppend, Values: st.Values,
	})
	reply, err := rt.External.RunTurn(ctx, in, rt.Tools, emit)
	if err != nil {
		return err
	}
	st.Reply = reply
	return nil
}

// PersistTurn 落 assistant 气泡；汇集本轮工具失败进 Values。学习在 Learn 步。
func (rt *Runtime) PersistTurn(ctx context.Context, st *lifecycle.RunState) error {
	_ = ctx
	if rt == nil || st == nil {
		return fmt.Errorf("defaults: nil runtime/state")
	}
	root := st.Root
	if root == "" {
		root = rt.Root()
		st.Root = root
	}
	if rt.ToolGateway != nil {
		if errs := rt.ToolGateway.DrainToolErrors(); len(errs) > 0 {
			st.Set(memory.ToolErrorsValueKey, errs)
		}
	}
	reply := strings.TrimSpace(st.Reply)
	// 外置大脑：工具轨迹须夹在 tool_calls 与正文之间，由 Client 落盘。
	if reply != "" && guest.ThinkMode(st.Values) != guest.ThinkExternal {
		if _, err := rt.Session.Append(root, "", st.SessionKey, "assistant", reply, st.TaskID, ""); err != nil {
			return err
		}
	}
	return nil
}

// Learn Evidence→Candidate→Policy→可选 MD Commit（默认 NoopPolicy）。
func (rt *Runtime) Learn(ctx context.Context, st *lifecycle.RunState) error {
	if rt == nil || st == nil {
		return fmt.Errorf("defaults: nil runtime/state")
	}
	learner := rt.Learner
	if learner == nil {
		return nil
	}
	root := st.Root
	if root == "" {
		root = rt.Root()
		st.Root = root
	}
	notes := memory.NotesFromToolErrors(memory.ToolErrorsFromValues(st.Values))
	reply := strings.TrimSpace(st.Reply)
	if reply == "" && notes == "" {
		return nil
	}
	res, err := learner.Run(ctx, learning.Evidence{
		Root:        root,
		SessionKey:  st.SessionKey,
		TaskID:      st.TaskID,
		Prompt:      st.Prompt,
		Reply:       reply,
		Feedforward: st.Feedforward,
		SkillIDs:    memory.SkillIDsFromValues(st.Values),
		Notes:       notes,
		Values:      st.Values,
	})
	if err != nil {
		return err
	}
	if res.Candidate.ID != "" {
		st.Set("learning.candidateId", res.Candidate.ID)
	}
	if res.Committed {
		st.Set("learning.committed", true)
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
