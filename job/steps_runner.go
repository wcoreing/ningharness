package job

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// runSteps 一条任务内串行多节；共用 task.SessionKey；进度写回 Steps。
func (m *Manager) runSteps(ctx context.Context, job Job) {
	var lastRunID string
	for {
		stepIdx, stepJob, ok := m.beginNextStep(job.ID)
		if !ok {
			m.finishJobAfterSteps(job.ID, lastRunID, nil, false)
			return
		}
		if ctx.Err() != nil {
			m.finishJobAfterSteps(job.ID, lastRunID, ctx.Err(), true)
			return
		}
		runID, err := m.exec(ctx, stepJob)
		lastRunID = runID
		if err != nil || ctx.Err() != nil {
			m.failStep(job.ID, stepIdx, runID, err, ctx.Err() != nil)
			return
		}
		m.completeStep(job.ID, stepIdx, runID)
	}
}

func (m *Manager) beginNextStep(taskID string) (stepIdx int, stepTask Job, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID != taskID {
			continue
		}
		t := &m.file.Jobs[i]
		if t.Status == StatusCancelled {
			return -1, Job{}, false
		}
		for j := range t.Steps {
			if t.Steps[j].Status == StepDone {
				continue
			}
			t.Steps[j].Status = StepRunning
			t.Steps[j].Error = ""
			t.TargetRel = t.Steps[j].Rel
			t.StepDone = countStepDone(t.Steps)
			t.StepTotal = len(t.Steps)
			t.ProgressHint = progressHintFor(t, j)
			if strings.TrimSpace(t.SessionKey) == "" {
				t.SessionKey = QueueSessionKey(t.ID)
			}
			stepTask = *t
			stepTask.Prompt = BuildStepUserPrompt(*t, j)
			stepTask.WirePrompt = ""
			stepTask.TargetRel = t.Steps[j].Rel
			_ = saveFile(m.root, m.file)
			m.emitLocked()
			return j, stepTask, true
		}
		return -1, Job{}, false
	}
	return -1, Job{}, false
}

func (m *Manager) completeStep(taskID string, stepIdx int, runID string) {
	m.mu.Lock()
	var done Job
	ok := false
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID != taskID {
			continue
		}
		t := &m.file.Jobs[i]
		if stepIdx >= 0 && stepIdx < len(t.Steps) {
			t.Steps[stepIdx].Status = StepDone
			t.Steps[stepIdx].TaskID = strings.TrimSpace(runID)
			t.Steps[stepIdx].Error = ""
		}
		t.TaskID = strings.TrimSpace(runID)
		t.StepDone = countStepDone(t.Steps)
		t.StepTotal = len(t.Steps)
		t.ProgressHint = fmt.Sprintf("已完成 %d/%d", t.StepDone, t.StepTotal)
		_ = saveFile(m.root, m.file)
		m.emitLocked()
		done = cloneJob(*t)
		ok = true
		break
	}
	m.mu.Unlock()
	// 最后一节由 finishJobAfterSteps 发 ProgressDone，避免「进度 n/n」与「完成」连刷两条。
	if ok && done.StepDone < done.StepTotal {
		m.fireProgress(done, ProgressStep)
	}
}

func (m *Manager) failStep(taskID string, stepIdx int, runID string, err error, cancelled bool) {
	m.mu.Lock()
	var done Job
	var kind ProgressKind
	delete(m.running, taskID)
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID != taskID {
			continue
		}
		t := &m.file.Jobs[i]
		if t.Status == StatusCancelled {
			break
		}
		t.TaskID = strings.TrimSpace(runID)
		t.FinishedAt = time.Now().UnixMilli()
		msg := "cancelled"
		if cancelled || (err != nil && strings.Contains(err.Error(), "context canceled")) {
			t.Status = StatusCancelled
			t.Error = msg
			kind = ProgressCancelled
		} else {
			t.Status = StatusError
			if err != nil {
				msg = err.Error()
			} else {
				msg = "step failed"
			}
			t.Error = msg
			t.LastError = msg
			kind = ProgressError
			if m.file.PauseOnError {
				m.file.Paused = true
				m.file.PauseReason = "上一任务失败：" + trimReason(msg, 120)
			}
		}
		if stepIdx >= 0 && stepIdx < len(t.Steps) {
			if t.Status == StatusCancelled {
				t.Steps[stepIdx].Status = StepPending
			} else {
				t.Steps[stepIdx].Status = StepError
			}
			t.Steps[stepIdx].TaskID = strings.TrimSpace(runID)
			t.Steps[stepIdx].Error = msg
		}
		t.StepDone = countStepDone(t.Steps)
		t.StepTotal = len(t.Steps)
		t.ProgressHint = fmt.Sprintf("失败 · %d/%d", t.StepDone, t.StepTotal)
		m.flushOrphanSteerLocked(t)
		done = cloneJob(*t)
		break
	}
	_ = saveFile(m.root, m.file)
	m.emitLocked()
	if !m.file.Paused {
		m.requestKickLocked()
	}
	m.mu.Unlock()
	if kind != "" {
		m.fireProgress(done, kind)
	}
}

func (m *Manager) finishJobAfterSteps(taskID, lastRunID string, err error, cancelled bool) {
	m.mu.Lock()
	var done Job
	var kind ProgressKind
	delete(m.running, taskID)
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID != taskID {
			continue
		}
		t := &m.file.Jobs[i]
		if t.Status == StatusCancelled {
			break
		}
		t.TaskID = strings.TrimSpace(lastRunID)
		t.FinishedAt = time.Now().UnixMilli()
		t.StepDone = countStepDone(t.Steps)
		t.StepTotal = len(t.Steps)
		if cancelled || err != nil {
			if cancelled {
				t.Status = StatusCancelled
				t.Error = "cancelled"
				kind = ProgressCancelled
			} else {
				t.Status = StatusError
				t.Error = err.Error()
				t.LastError = t.Error
				kind = ProgressError
				if m.file.PauseOnError {
					m.file.Paused = true
					m.file.PauseReason = "上一任务失败：" + trimReason(err.Error(), 120)
				}
			}
			t.ProgressHint = fmt.Sprintf("中断 · %d/%d", t.StepDone, t.StepTotal)
		} else {
			t.Status = StatusDone
			t.Error = ""
			kind = ProgressDone
			t.ProgressHint = fmt.Sprintf("完成 · %d/%d", t.StepDone, t.StepTotal)
		}
		m.flushOrphanSteerLocked(t)
		done = cloneJob(*t)
		break
	}
	_ = saveFile(m.root, m.file)
	m.emitLocked()
	if !m.file.Paused {
		m.requestKickLocked()
	}
	m.mu.Unlock()
	if kind != "" {
		m.fireProgress(done, kind)
	}
}
