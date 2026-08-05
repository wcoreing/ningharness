package lifecycle

import "context"

// RunState 一轮生命周期的管道上下文（Turn Context）——本轮唯一真相。
// 步骤与 Bus 事件共享同一份；Gateway 投影经 ProjectTurn / FinishTurn 同步（见 defaults.BeginTask OnExit）。
// WithRunState 仅供 Host/defaults 同进程步骤读取；Guest 不依赖、也不应感知 RunState。
type RunState struct {
	Root       string
	SessionKey string
	TaskID     string
	JobID      string
	Prompt     string
	Reply      string

	// Feedforward 写入 user 行的前馈（进模 WireUser）；空则不写。
	Feedforward string

	// SkipUserAppend 为 true 时 assemble_context 不落 user 气泡（Job 且无 Feedforward 时常用）。
	SkipUserAppend bool

	// Values 步骤间旁路数据（扩展用）。
	Values map[string]any

	cleanups []func(error)
}

// Context 是 RunState 的别名，强调「管道上下文」。
type Context = RunState

type ctxKey int

const runStateKey ctxKey = 1

// WithRunState 将本轮状态挂入 ctx（Runner / Host 步骤用）。
// 不要指望经 Guest 框架再传回工具层；工具侧本轮身份以 Gateway 投影为准。
func WithRunState(ctx context.Context, st *RunState) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if st == nil {
		return ctx
	}
	return context.WithValue(ctx, runStateKey, st)
}

// RunStateFrom 从 ctx 取本轮状态；无则 nil。
func RunStateFrom(ctx context.Context) *RunState {
	if ctx == nil {
		return nil
	}
	st, _ := ctx.Value(runStateKey).(*RunState)
	return st
}

// OnExit 注册 LIFO 清理（Runner 在 Run 返回前必调，成功或失败均执行）。
// 投影与 Trace 收尾应挂在这里（唯一收尾路径），不要在 end_task 再 Disarm 一次。
func (s *RunState) OnExit(fn func(error)) {
	if s == nil || fn == nil {
		return
	}
	s.cleanups = append(s.cleanups, fn)
}

func (s *RunState) runCleanups(err error) {
	if s == nil {
		return
	}
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		s.cleanups[i](err)
	}
	s.cleanups = nil
}

// Set 写入 Values。
func (s *RunState) Set(key string, v any) {
	if s == nil {
		return
	}
	if s.Values == nil {
		s.Values = map[string]any{}
	}
	s.Values[key] = v
}

// Get 读取 Values。
func (s *RunState) Get(key string) (any, bool) {
	if s == nil || s.Values == nil {
		return nil, false
	}
	v, ok := s.Values[key]
	return v, ok
}
