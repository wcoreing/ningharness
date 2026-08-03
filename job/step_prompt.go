package job

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BuildStepUserPrompt 批内当前节进会话气泡 / Agent 主任务：仅用户模板（可选显式 {path} 占位）。
// 树勾选路径在 queue.json steps 里，不拼进 Prompt；Agent 用 list_queue + batch-write Skill 读取。
func BuildStepUserPrompt(job Job, stepIdx int) string {
	if stepIdx < 0 || stepIdx >= len(job.Steps) {
		return strings.TrimSpace(job.Prompt)
	}
	step := job.Steps[stepIdx]
	rel := strings.TrimSpace(step.Rel)
	tpl := strings.TrimSpace(step.Prompt)
	if tpl == "" {
		tpl = strings.TrimSpace(job.Prompt)
	}
	if tpl == "" {
		tpl = DefaultPathPrompt
	}
	if strings.Contains(tpl, "{path}") {
		return strings.TrimSpace(strings.ReplaceAll(tpl, "{path}", rel))
	}
	return strings.TrimSpace(tpl)
}

// BuildStepWirePrompt 等同用户主任务。
func BuildStepWirePrompt(job Job, stepIdx int) string {
	return BuildStepUserPrompt(job, stepIdx)
}

// BuildStepPrompt 兼容旧名。
func BuildStepPrompt(job Job, stepIdx int) string {
	return BuildStepUserPrompt(job, stepIdx)
}

func countStepDone(steps []Step) int {
	n := 0
	for _, s := range steps {
		if s.Status == StepDone {
			n++
		}
	}
	return n
}

func resetIncompleteSteps(t *Job) {
	if t == nil || len(t.Steps) == 0 {
		return
	}
	for i := range t.Steps {
		if t.Steps[i].Status == StepDone {
			continue
		}
		t.Steps[i].Status = StepPending
		t.Steps[i].TaskID = ""
		t.Steps[i].Error = ""
	}
	t.StepDone = countStepDone(t.Steps)
	t.StepTotal = len(t.Steps)
	t.ProgressHint = fmt.Sprintf("排队 · 已完成 %d/%d", t.StepDone, t.StepTotal)
}

func progressHintFor(t *Job, stepIdx int) string {
	if t == nil || len(t.Steps) == 0 {
		return ""
	}
	done := countStepDone(t.Steps)
	title := ""
	if stepIdx >= 0 && stepIdx < len(t.Steps) {
		title = strings.TrimSpace(t.Steps[stepIdx].Title)
		if title == "" {
			title = filepath.Base(t.Steps[stepIdx].Rel)
		}
	}
	if title != "" {
		return fmt.Sprintf("%d/%d · %s", done+1, len(t.Steps), title)
	}
	return fmt.Sprintf("%d/%d", done, len(t.Steps))
}
