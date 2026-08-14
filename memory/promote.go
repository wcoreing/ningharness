package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/wcoreing/ningharness/lesson"
)

const (
	// promoteBodyMaxRunes 单条晋升 lesson 正文上限。
	promoteBodyMaxRunes = 600
	// MaxPromotePerTurn 单轮最多晋升条数（防刷）。
	MaxPromotePerTurn = 3
)

// ToolErrorPromoter 将本轮工具失败 Notes 晋升为 project-scope lesson（候选经验，写入即进前馈）。
// 同错误指纹已存在 active lesson 时跳过。
type ToolErrorPromoter struct{}

// Ingest 解析 Notes 中的 tool_errors 列表并 Append（去重）。
func (p ToolErrorPromoter) Ingest(ctx context.Context, in IngestInput) error {
	_ = ctx
	_ = p
	root := strings.TrimSpace(in.Root)
	if root == "" {
		return nil
	}
	errs := parseToolErrorNotes(in.Notes)
	if len(errs) == 0 {
		return nil
	}
	if len(errs) > MaxPromotePerTurn {
		errs = errs[:MaxPromotePerTurn]
	}
	for _, e := range errs {
		needle := promoteNeedle(e)
		if needle == "" {
			continue
		}
		if lesson.HasActiveContaining(root, lesson.ScopeProject, needle) {
			continue
		}
		body := formatPromoteBody(e, in.Prompt)
		if body == "" {
			continue
		}
		if _, err := lesson.Append(lesson.AppendInput{
			Root:             root,
			Scope:            lesson.ScopeProject,
			Body:             body,
			SourceTaskID:     in.TaskID,
			SourceSessionKey: in.SessionKey,
		}); err != nil {
			return fmt.Errorf("memory promote: %w", err)
		}
	}
	return nil
}

// PromoteGoalBlocked 将 Goal 受阻原因晋升为 project lesson（去重）。
func PromoteGoalBlocked(root, reason, objective, taskID, sessionKey string) error {
	root = strings.TrimSpace(root)
	reason = strings.TrimSpace(reason)
	if root == "" || reason == "" {
		return nil
	}
	needle := "[goal_blocked] " + reason
	if lesson.HasActiveContaining(root, lesson.ScopeProject, needle) {
		return nil
	}
	var b strings.Builder
	b.WriteString(needle)
	if obj := strings.TrimSpace(objective); obj != "" {
		b.WriteString("\nobjective: ")
		b.WriteString(trimRunes(obj, 160))
	}
	_, err := lesson.Append(lesson.AppendInput{
		Root:             root,
		Scope:            lesson.ScopeProject,
		Body:             trimRunes(b.String(), promoteBodyMaxRunes),
		SourceTaskID:     taskID,
		SourceSessionKey: sessionKey,
	})
	if err != nil {
		return fmt.Errorf("memory promote goal: %w", err)
	}
	return nil
}

func promoteNeedle(toolErr string) string {
	toolErr = strings.TrimSpace(toolErr)
	if toolErr == "" {
		return ""
	}
	return "[tool_error] " + toolErr
}

func parseToolErrorNotes(notes string) []string {
	notes = strings.TrimSpace(notes)
	if notes == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(notes, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimSpace(line[2:])
		}
		if line == "" || line == "tool_errors:" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func formatPromoteBody(toolErr, prompt string) string {
	toolErr = strings.TrimSpace(toolErr)
	if toolErr == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(promoteNeedle(toolErr))
	if p := strings.TrimSpace(prompt); p != "" {
		b.WriteString("\ncontext: ")
		b.WriteString(trimRunes(p, 160))
	}
	return trimRunes(b.String(), promoteBodyMaxRunes)
}

// MultiIngest 顺序调用多个 Ingester。
type MultiIngest []Ingester

// Ingest 依次沉淀；任一失败即返回。
func (m MultiIngest) Ingest(ctx context.Context, in IngestInput) error {
	for _, ing := range m {
		if ing == nil {
			continue
		}
		if err := ing.Ingest(ctx, in); err != nil {
			return err
		}
	}
	return nil
}

// NewLessonWithPromotion 默认 Lesson 前馈 + JSONL 沉淀 + 工具失败晋升 project lesson。
func NewLessonWithPromotion() Bundle {
	return Bundle{
		Memory: NewLesson(),
		Ingester: MultiIngest{
			&FileIngest{},
			ToolErrorPromoter{},
		},
	}
}

var _ Ingester = ToolErrorPromoter{}
var _ Ingester = MultiIngest{}
