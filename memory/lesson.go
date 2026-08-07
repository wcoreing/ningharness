package memory

import (
	"context"

	"ningharness/lesson"
)

// Lesson 默认 Memory：用 lesson.InjectBrief 贡献前馈。
type Lesson struct {
	// MaxRunes 注入上限；≤0 则用 lesson.InjectMaxRunes。
	MaxRunes int
}

// NewLesson 返回默认 Lesson Memory。
func NewLesson() *Lesson {
	return &Lesson{}
}

// Assemble 返回 lesson 经验摘要片段。
func (m *Lesson) Assemble(ctx context.Context, in AssembleInput) (string, error) {
	_ = ctx
	if m == nil {
		return "", nil
	}
	root := in.Root
	if root == "" {
		return "", nil
	}
	max := m.MaxRunes
	if max <= 0 {
		max = lesson.InjectMaxRunes
	}
	return lesson.InjectBrief(root, in.SkillIDs, max), nil
}

// Ingest 默认 no-op（候选沉淀由客户端或后续实现）。
func (m *Lesson) Ingest(ctx context.Context, in IngestInput) error {
	_ = ctx
	_ = in
	_ = m
	return nil
}

var _ Memory = (*Lesson)(nil)
var _ Ingester = (*Lesson)(nil)
