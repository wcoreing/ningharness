package job

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	enqueueOKPrefix = "已入队（未落盘）"
	maxPromptBrief  = 80
)

// FormatEnqueueOK 入队成功回执：明确「入队 ≠ 落盘」。
func FormatEnqueueOK(job Job) string {
	id := strings.TrimSpace(job.ID)
	st := string(job.Status)
	if st == "" {
		st = string(StatusQueued)
	}
	msg := fmt.Sprintf("%s · id=%s status=%s", enqueueOKPrefix, id, st)
	if rel := strings.TrimSpace(job.TargetRel); rel != "" {
		msg += " · target=`" + rel + "`（执行时注入「本轮只写」）"
	}
	if job.StepTotal > 1 {
		msg += fmt.Sprintf(" · steps=%d", job.StepTotal)
	} else if len(job.Steps) > 1 {
		msg += fmt.Sprintf(" · steps=%d", len(job.Steps))
	}
	msg += " · 正文由队列节 write_file/edit；list_queue 看进度；见写盘成功回执才可说已落盘"
	return msg
}

// AgentStepBrief Skill/MCP 可读节（不含 prompt 全文）。
type AgentStepBrief struct {
	Rel    string `json:"rel"`
	Title  string `json:"title,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// AgentJobBrief Skill/MCP 可读任务摘要。
type AgentJobBrief struct {
	ID           string           `json:"id"`
	Title        string           `json:"title,omitempty"`
	Status       string           `json:"status"`
	TargetRel    string           `json:"targetRel,omitempty"`
	ProgressHint string           `json:"progressHint,omitempty"`
	StepDone     int              `json:"stepDone,omitempty"`
	StepTotal    int              `json:"stepTotal,omitempty"`
	Steps        []AgentStepBrief `json:"steps,omitempty"`
	Error        string           `json:"error,omitempty"`
	PromptBrief  string           `json:"promptBrief,omitempty"`
}

// AgentSnapshot list_queue 给 Agent 的结构化快照（无 prompt/FeedExtra 全文）。
type AgentSnapshot struct {
	Note         string          `json:"note"`
	Paused       bool            `json:"paused"`
	PauseOnError bool            `json:"pauseOnError"`
	PauseReason  string          `json:"pauseReason,omitempty"`
	MaxParallel  int             `json:"maxParallel"`
	Stats        Stats           `json:"stats"`
	Jobs         []AgentJobBrief `json:"jobs"`
}

// FormatAgentSnapshot 压缩 Snapshot，供 list_queue。
func FormatAgentSnapshot(snap Snapshot) AgentSnapshot {
	out := AgentSnapshot{
		Note:         "入队成功≠落盘；targetRel=执行节「本轮只写」；status=done 仍须 list_tree/read_file 或写盘回执确认正文。jobs 不含 prompt 全文。",
		Paused:       snap.Paused,
		PauseOnError: snap.PauseOnError,
		PauseReason:  snap.PauseReason,
		MaxParallel:  snap.MaxParallel,
		Stats:        snap.Stats,
		Jobs:         make([]AgentJobBrief, 0, len(snap.Jobs)),
	}
	for _, j := range snap.Jobs {
		out.Jobs = append(out.Jobs, agentJobBrief(j))
	}
	return out
}

func agentJobBrief(j Job) AgentJobBrief {
	b := AgentJobBrief{
		ID:           j.ID,
		Title:        j.Title,
		Status:       string(j.Status),
		TargetRel:    j.TargetRel,
		ProgressHint: j.ProgressHint,
		StepDone:     j.StepDone,
		StepTotal:    j.StepTotal,
		Error:        firstNonEmpty(j.Error, j.LastError),
		PromptBrief:  trimPromptBrief(j.Prompt),
	}
	if len(j.Steps) > 0 {
		b.Steps = make([]AgentStepBrief, 0, len(j.Steps))
		for _, s := range j.Steps {
			b.Steps = append(b.Steps, AgentStepBrief{
				Rel:    s.Rel,
				Title:  s.Title,
				Status: string(s.Status),
				Error:  s.Error,
			})
		}
		if b.StepTotal == 0 {
			b.StepTotal = len(j.Steps)
		}
		if b.TargetRel == "" {
			b.TargetRel = j.Steps[0].Rel
		}
	}
	return b
}

func trimPromptBrief(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, "\n", " ")
	if utf8.RuneCountInString(p) <= maxPromptBrief {
		return p
	}
	r := []rune(p)
	return string(r[:maxPromptBrief]) + "…"
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

// NormRel 队列路径归一（与入队一致）。
func NormRel(rel string) string {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	rel = strings.TrimPrefix(rel, "./")
	return filepath.ToSlash(rel)
}

// ProgressKind 隐藏会话任务写回侧栏的进度阶段。
type ProgressKind string

const (
	ProgressStep      ProgressKind = "step"
	ProgressDone      ProgressKind = "done"
	ProgressError     ProgressKind = "error"
	ProgressCancelled ProgressKind = "cancelled"
)

// FormatSidebarEnqueue 隐藏队列入队时侧栏短话术（assistant）。
func FormatSidebarEnqueue(job Job) string {
	n := job.StepTotal
	if n == 0 {
		n = len(job.Steps)
	}
	if n > 1 {
		return fmt.Sprintf("已入队批写 %d 节（%s）· 后台执行，进度会更新在此", n, pathListBrief(job, 4))
	}
	if rel := strings.TrimSpace(job.TargetRel); rel != "" {
		return fmt.Sprintf("已入队后台任务 · `%s` · 完成后会在此告知", rel)
	}
	return fmt.Sprintf("已入队后台任务 · id=%s · 完成后会在此告知", strings.TrimSpace(job.ID))
}

// FormatSidebarProgress 隐藏队列执行进度/终态侧栏短话术。
func FormatSidebarProgress(job Job, kind ProgressKind) string {
	total := job.StepTotal
	if total == 0 {
		total = len(job.Steps)
	}
	done := job.StepDone
	switch kind {
	case ProgressStep:
		rel := lastDoneRel(job)
		if rel != "" {
			return fmt.Sprintf("批写进度 %d/%d · 已完成 `%s`", done, total, rel)
		}
		return fmt.Sprintf("批写进度 %d/%d", done, total)
	case ProgressDone:
		if total > 1 {
			return fmt.Sprintf("批写完成 %d/%d · %s", done, total, pathListBrief(job, 6))
		}
		if rel := strings.TrimSpace(job.TargetRel); rel != "" {
			return fmt.Sprintf("队列任务完成 · `%s`", rel)
		}
		return "队列任务完成"
	case ProgressCancelled:
		if total > 1 {
			return fmt.Sprintf("批写已取消 · %d/%d", done, total)
		}
		return "队列任务已取消"
	case ProgressError:
		err := firstNonEmpty(job.Error, job.LastError)
		if total > 1 {
			if err != "" {
				return fmt.Sprintf("批写中断 %d/%d · %s", done, total, trimReason(err, 80))
			}
			return fmt.Sprintf("批写中断 %d/%d", done, total)
		}
		if err != "" {
			return "队列任务失败 · " + trimReason(err, 80)
		}
		return "队列任务失败"
	default:
		return ""
	}
}

func lastDoneRel(job Job) string {
	for i := len(job.Steps) - 1; i >= 0; i-- {
		if job.Steps[i].Status == StepDone {
			if r := strings.TrimSpace(job.Steps[i].Rel); r != "" {
				return r
			}
		}
	}
	return strings.TrimSpace(job.TargetRel)
}

func pathListBrief(job Job, max int) string {
	if max < 1 {
		max = 1
	}
	rels := make([]string, 0, len(job.Steps))
	for _, s := range job.Steps {
		if r := strings.TrimSpace(s.Rel); r != "" {
			rels = append(rels, "`"+r+"`")
		}
	}
	if len(rels) == 0 {
		if r := strings.TrimSpace(job.TargetRel); r != "" {
			return "`" + r + "`"
		}
		return "—"
	}
	if len(rels) <= max {
		return strings.Join(rels, "、")
	}
	return strings.Join(rels[:max], "、") + fmt.Sprintf(" 等 %d 个", len(rels))
}
