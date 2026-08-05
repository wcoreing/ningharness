package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Moment 生命周期阶段边界（before / after）。
// 事件载荷为同一份 RunState；本文件只负责打孔，不拥有世界 I/O。
type Moment int

const (
	Before Moment = iota
	After
)

func (m Moment) String() string {
	switch m {
	case Before:
		return "before"
	case After:
		return "after"
	default:
		return fmt.Sprintf("moment(%d)", int(m))
	}
}

// Event 生命周期事件：固定阶段打孔，载荷为本轮管道上下文（State）。
type Event struct {
	Phase  StepID
	Moment Moment
	State  *RunState
}

// Handler 事件处理；On（Gate）返回 ErrBlock 中止本轮；Watch 错误不拖死管道。
type Handler func(ctx context.Context, ev Event) error

type subscription struct {
	watch bool
	h     Handler
}

// Bus 生命周期事件总线（闭集 Phase × Before/After）。
type Bus struct {
	mu   sync.Mutex
	subs map[StepID]map[Moment][]subscription
}

// NewBus 空总线。
func NewBus() *Bus {
	return &Bus{subs: map[StepID]map[Moment][]subscription{}}
}

func (b *Bus) ensure(phase StepID, m Moment) {
	if b.subs == nil {
		b.subs = map[StepID]map[Moment][]subscription{}
	}
	if b.subs[phase] == nil {
		b.subs[phase] = map[Moment][]subscription{}
	}
}

// On 注册 Gate：可 Block / 改 State；错误中止 Emit。
func (b *Bus) On(phase StepID, m Moment, h Handler) {
	if b == nil || h == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ensure(phase, m)
	b.subs[phase][m] = append(b.subs[phase][m], subscription{watch: false, h: h})
}

// Watch 注册观察者：不可作为控制面；错误忽略。
func (b *Bus) Watch(phase StepID, m Moment, h Handler) {
	if b == nil || h == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ensure(phase, m)
	b.subs[phase][m] = append(b.subs[phase][m], subscription{watch: true, h: h})
}

// Clone 复制订阅列表（Handler 闭包共享）。
func (b *Bus) Clone() *Bus {
	if b == nil {
		return NewBus()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := NewBus()
	for phase, byM := range b.subs {
		for m, list := range byM {
			if len(list) == 0 {
				continue
			}
			out.ensure(phase, m)
			out.subs[phase][m] = append([]subscription(nil), list...)
		}
	}
	return out
}

// Emit 先跑全部 Gate，再跑 Watch。Gate 遇 ErrBlock/其它错误立即返回。
func (b *Bus) Emit(ctx context.Context, ev Event) error {
	if b == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	b.mu.Lock()
	var gates, watches []Handler
	if byM := b.subs[ev.Phase]; byM != nil {
		for _, s := range byM[ev.Moment] {
			if s.h == nil {
				continue
			}
			if s.watch {
				watches = append(watches, s.h)
			} else {
				gates = append(gates, s.h)
			}
		}
	}
	b.mu.Unlock()

	for _, h := range gates {
		if err := h(ctx, ev); err != nil {
			if errors.Is(err, ErrBlock) {
				return err
			}
			return fmt.Errorf("lifecycle: gate %s.%s: %w", ev.Phase, ev.Moment, err)
		}
	}
	for _, h := range watches {
		_ = h(ctx, ev) // Watch 不拖死
	}
	return nil
}
