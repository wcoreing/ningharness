// Package lifecycle 定义 Harness「一轮」怎么跑：固定领域步骤 + 事件打孔 + 管道上下文。
//
// 层职责：本包装生命周期与 RunState；不写盘、不注册 MCP 工具。
// 工具只发生在 run_guest 内（经 toolgateway）；步骤实现由 defaults.Host 注入，避免本包依赖 toolgateway。
package lifecycle

import (
	"context"
	"errors"
	"fmt"
)

// StepID 默认生命周期步骤 id（闭集）。
type StepID string

const (
	StepBeginTask StepID = "begin_task"
	// StepAssembleContext 宿主侧输入面（默认落 user；可 Gate 加厚 feedforward 等）。
	StepAssembleContext StepID = "assemble_context"
	StepRunGuest        StepID = "run_guest"
	StepPersistTurn     StepID = "persist_turn"
	// StepEndTask Bus 钩子点；投影/Trace 收尾在 RunState.OnExit→FinishTurn。
	StepEndTask StepID = "end_task"
)

// ErrBlock Gate/Before 返回此错误（或 errors.Is）时中止生命周期，不跑本步 Run。
var ErrBlock = errors.New("lifecycle: blocked")

// Action 步骤前后动作（糖衣；内部转到 Bus.On）。
type Action func(ctx context.Context, st *RunState) error

// Step 生命周期领域原子步骤。
type Step struct {
	ID  StepID
	Run func(ctx context.Context, st *RunState) error
}

// Lifecycle 有序步骤表 + 生命周期事件总线。
type Lifecycle struct {
	steps []Step
	bus   *Bus
}

// New 从步骤列表构造（自带空 Bus）。
func New(steps ...Step) *Lifecycle {
	return &Lifecycle{
		steps: append([]Step(nil), steps...),
		bus:   NewBus(),
	}
}

// Bus 生命周期事件总线（可直接 On/Watch）。
func (w *Lifecycle) Bus() *Bus {
	if w == nil {
		return nil
	}
	if w.bus == nil {
		w.bus = NewBus()
	}
	return w.bus
}

// On 在阶段边界注册 Gate（可 Block）。
func (w *Lifecycle) On(phase StepID, m Moment, h Handler) *Lifecycle {
	if w == nil {
		return w
	}
	w.Bus().On(phase, m, h)
	return w
}

// Watch 在阶段边界注册观察者（错误不中止）。
func (w *Lifecycle) Watch(phase StepID, m Moment, h Handler) *Lifecycle {
	if w == nil {
		return w
	}
	w.Bus().Watch(phase, m, h)
	return w
}

// Steps 返回步骤副本。
func (w *Lifecycle) Steps() []Step {
	if w == nil {
		return nil
	}
	return append([]Step(nil), w.steps...)
}

// Clone 深拷贝步骤与总线订阅（Run/Handler 闭包共享）。
func (w *Lifecycle) Clone() *Lifecycle {
	if w == nil {
		return nil
	}
	out := New(w.steps...)
	out.bus = w.Bus().Clone()
	return out
}

// WithBefore 追加某步 Before Gate（糖衣 → Bus.On Before）。
func (w *Lifecycle) WithBefore(id StepID, acts ...Action) *Lifecycle {
	if w == nil || len(acts) == 0 {
		return w
	}
	for _, a := range acts {
		if a == nil {
			continue
		}
		act := a
		w.Bus().On(id, Before, func(ctx context.Context, ev Event) error {
			return act(ctx, ev.State)
		})
	}
	return w
}

// WithAfter 追加某步 After Gate。
func (w *Lifecycle) WithAfter(id StepID, acts ...Action) *Lifecycle {
	if w == nil || len(acts) == 0 {
		return w
	}
	for _, a := range acts {
		if a == nil {
			continue
		}
		act := a
		w.Bus().On(id, After, func(ctx context.Context, ev Event) error {
			return act(ctx, ev.State)
		})
	}
	return w
}

// InsertAfter 在锚点步骤后插入一步；锚点不存在则报错。
func (w *Lifecycle) InsertAfter(after StepID, step Step) error {
	if w == nil {
		return fmt.Errorf("lifecycle: nil")
	}
	if step.ID == "" {
		return fmt.Errorf("lifecycle: empty step id")
	}
	for i, s := range w.steps {
		if s.ID == after {
			w.steps = append(w.steps[:i+1], append([]Step{step}, w.steps[i+1:]...)...)
			return nil
		}
	}
	return fmt.Errorf("lifecycle: step %q not found", after)
}

// Replace 替换同 id 步骤的 Run（保留事件订阅）。
func (w *Lifecycle) Replace(id StepID, run func(ctx context.Context, st *RunState) error) error {
	if w == nil {
		return fmt.Errorf("lifecycle: nil")
	}
	for i := range w.steps {
		if w.steps[i].ID == id {
			w.steps[i].Run = run
			return nil
		}
	}
	return fmt.Errorf("lifecycle: step %q not found", id)
}

// Host 默认生命周期各步的宿主实现（由 defaults 注入，避免循环依赖）。
type Host interface {
	BeginTask(ctx context.Context, st *RunState) error
	AssembleContext(ctx context.Context, st *RunState) error
	RunGuest(ctx context.Context, st *RunState) error
	PersistTurn(ctx context.Context, st *RunState) error
	EndTask(ctx context.Context, st *RunState) error
}

// NewDefault 返回默认生命周期（begin_task → … → end_task），Run 绑定到 Host。
func NewDefault(h Host) *Lifecycle {
	if h == nil {
		return New()
	}
	return New(
		Step{ID: StepBeginTask, Run: h.BeginTask},
		Step{ID: StepAssembleContext, Run: h.AssembleContext},
		Step{ID: StepRunGuest, Run: h.RunGuest},
		Step{ID: StepPersistTurn, Run: h.PersistTurn},
		Step{ID: StepEndTask, Run: h.EndTask},
	)
}

// Runner 执行生命周期：每步 Bus Emit(before) → run → Emit(after)。
type Runner struct{}

// Run 按序执行。
// Before Gate 返回 ErrBlock：中止且不跑本步 Run/After。
// Run 失败：跳过本步 After，停止后续步骤。
// 已成功步骤的 After 已执行；Run 返回前必跑 state.OnExit 清理。
func (Runner) Run(ctx context.Context, wf *Lifecycle, st *RunState) (err error) {
	if st == nil {
		return fmt.Errorf("lifecycle: nil state")
	}
	defer func() { st.runCleanups(err) }()
	if wf == nil {
		return fmt.Errorf("lifecycle: nil lifecycle")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	bus := wf.Bus()

	for _, step := range wf.steps {
		ev := Event{Phase: step.ID, Moment: Before, State: st}
		if berr := bus.Emit(ctx, ev); berr != nil {
			return berr
		}
		if step.Run == nil {
			return fmt.Errorf("lifecycle: step %s: nil Run", step.ID)
		}
		if rerr := step.Run(ctx, st); rerr != nil {
			return fmt.Errorf("lifecycle: run %s: %w", step.ID, rerr)
		}
		ev.Moment = After
		if aerr := bus.Emit(ctx, ev); aerr != nil {
			return aerr
		}
	}
	return nil
}
