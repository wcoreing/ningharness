// Package memory 是 Memory 插槽：在 assemble 时贡献前馈；可选 Ingest 回合后沉淀。
// 不拥有 Lifecycle / Gateway；持久行在 Store（lesson / history / resource）。
package memory

import (
	"context"
	"strings"
)

// SkillIDsValueKey 可写入 RunState.Values，供 assemble 传入 Lesson 匹配。
const SkillIDsValueKey = "memory.skillIDs"

// AssembleInput Memory.Assemble 入参（窄于 RunState，避免本包依赖 lifecycle）。
type AssembleInput struct {
	Root                string
	SessionKey          string
	TaskID              string
	Prompt              string
	ExistingFeedforward string
	SkillIDs            []string
}

// IngestInput 回合落盘后可选沉淀入参。
type IngestInput struct {
	Root        string
	SessionKey  string
	TaskID      string
	Prompt      string
	Reply       string
	Feedforward string
	SkillIDs    []string
}

// Memory 可换记忆实现：返回应并入前馈的片段（追加，不覆盖 Existing）。
type Memory interface {
	Assemble(ctx context.Context, in AssembleInput) (patch string, err error)
}

// Ingester 可选：PersistTurn 之后沉淀（默认 Lesson 为 no-op）。
type Ingester interface {
	Ingest(ctx context.Context, in IngestInput) error
}

// MergeFeedforward 合并调用方前馈与 Memory 片段。
func MergeFeedforward(base, patch string) string {
	base = strings.TrimSpace(base)
	patch = strings.TrimSpace(patch)
	if base == "" {
		return patch
	}
	if patch == "" {
		return base
	}
	return base + "\n\n" + patch
}

// SkillIDsFromValues 从 RunState.Values 解析 skill id 列表。
func SkillIDsFromValues(values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	v, ok := values[SkillIDsValueKey]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return trimIDs(t)
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return trimIDs(out)
	default:
		return nil
	}
}

func trimIDs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
