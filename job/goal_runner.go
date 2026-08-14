package job

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/wcoreing/ningharness/goal"
	"github.com/wcoreing/ningharness/memory"
	"github.com/wcoreing/ningharness/metadir"
)

func goalControlPath(root, jobID string) string {
	return filepath.Join(metadir.ProjectDir(root), "goals", jobID, "GOAL.yaml")
}

// GoalControlPath 项目根下 GOAL.yaml 绝对路径。
func GoalControlPath(root, jobID string) string {
	return goalControlPath(root, jobID)
}

func (m *Manager) runGoal(ctx context.Context, job Job) {
	root := m.root
	control := goalControlPath(root, job.ID)
	planRel := filepath.ToSlash(filepath.Join(metadir.ProjectRel(root), "goals", job.ID, "PLAN.md"))
	maxRounds := job.GoalMaxRounds
	if maxRounds < 1 {
		maxRounds = DefaultGoalMaxRounds
	}
	var lastRunID string
	outcome, err := goal.Run(ctx, goal.Spec{
		Objective:   job.Prompt,
		ControlPath: control,
		PlanRel:     planRel,
		MaxRounds:   maxRounds,
	}, func(ctx context.Context, wire string, round int) error {
		m.mu.Lock()
		var roundJob Job
		ok := false
		for i := range m.file.Jobs {
			if m.file.Jobs[i].ID != job.ID {
				continue
			}
			t := &m.file.Jobs[i]
			if t.Status == StatusCancelled {
				m.mu.Unlock()
				return context.Canceled
			}
			t.GoalRound = round
			t.GoalMaxRounds = maxRounds
			t.ProgressHint = fmt.Sprintf("目标 · 第 %d 轮", round)
			if steer := m.takeSteerPendingLocked(job.ID); steer != "" {
				wire = wire + "\n\n" + FormatSteerBlock(steer)
				t.ProgressHint = fmt.Sprintf("目标 · 第 %d 轮 · 已注入插话", round)
			}
			roundJob = cloneJob(*t)
			roundJob.WirePrompt = wire
			roundJob.Prompt = wire
			_ = saveFile(m.root, m.file)
			m.emitLocked()
			ok = true
			break
		}
		m.mu.Unlock()
		if !ok {
			return fmt.Errorf("job: goal %s missing", job.ID)
		}
		m.fireProgress(roundJob, ProgressStep)
		runID, execErr := m.exec(ctx, roundJob)
		lastRunID = runID
		return execErr
	}, nil)

	m.finishGoal(job.ID, lastRunID, outcome, err, ctx.Err() != nil)
}

func (m *Manager) finishGoal(jobID, lastRunID string, outcome goal.Outcome, err error, cancelled bool) {
	m.mu.Lock()
	var done Job
	var kind ProgressKind
	var promoteRoot, promoteReason, promoteObj, promoteTask, promoteSess string
	delete(m.running, jobID)
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID != jobID {
			continue
		}
		t := &m.file.Jobs[i]
		if t.Status == StatusCancelled {
			break
		}
		t.TaskID = strings.TrimSpace(lastRunID)
		t.FinishedAt = time.Now().UnixMilli()
		abortCancel := cancelled || (outcome == goal.OutcomeAborted && err != nil &&
			(strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "context cancelled")))
		switch {
		case abortCancel:
			t.Status = StatusCancelled
			t.Error = "cancelled"
			kind = ProgressCancelled
			t.ProgressHint = fmt.Sprintf("目标中断 · 第 %d 轮", t.GoalRound)
		case outcome == goal.OutcomeComplete || outcome == goal.OutcomeBlocked:
			t.Status = StatusDone
			t.Error = ""
			kind = ProgressDone
			if outcome == goal.OutcomeBlocked {
				reason := goal.ReadReason(goalControlPath(m.root, jobID))
				if reason == "" {
					reason = "blocked（未写 reason）"
				}
				t.LastError = reason
				t.ProgressHint = fmt.Sprintf("目标受阻 · 第 %d 轮 · %s", t.GoalRound, trimReason(reason, 80))
				promoteRoot, promoteReason = m.root, reason
				promoteObj, promoteTask, promoteSess = t.Prompt, t.TaskID, t.SessionKey
			} else {
				t.ProgressHint = fmt.Sprintf("目标完成 · 第 %d 轮", t.GoalRound)
			}
		default:
			t.Status = StatusError
			msg := "goal failed"
			if err != nil {
				msg = err.Error()
			} else if outcome == goal.OutcomeMaxRounds {
				msg = fmt.Sprintf("goal: max rounds %d", t.GoalMaxRounds)
			}
			t.Error = msg
			t.LastError = msg
			kind = ProgressError
			t.ProgressHint = fmt.Sprintf("目标失败 · 第 %d 轮", t.GoalRound)
			if m.file.PauseOnError {
				m.file.Paused = true
				m.file.PauseReason = "上一任务失败：" + trimReason(msg, 120)
			}
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
	if promoteReason != "" {
		_ = memory.PromoteGoalBlocked(promoteRoot, promoteReason, promoteObj, promoteTask, promoteSess)
	}
	if kind != "" {
		m.fireProgress(done, kind)
	}
}

// ResumeGoal 将 blocked Goal 控制面改回 active，并把已结束的 Goal Job 重新入队续跑。
func (m *Manager) ResumeGoal(jobID string) (Job, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return Job{}, fmt.Errorf("job: empty task id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.TrimSpace(m.root) == "" {
		return Job{}, fmt.Errorf("job: no project")
	}
	idx := -1
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID == jobID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Job{}, fmt.Errorf("job: task not found")
	}
	t := &m.file.Jobs[idx]
	if t.Type != JobTypeGoal {
		return Job{}, fmt.Errorf("job: not a goal task (%s)", t.Type)
	}
	control := goalControlPath(m.root, jobID)
	if err := goal.Resume(control); err != nil {
		return Job{}, err
	}
	switch t.Status {
	case StatusDone, StatusError, StatusCancelled:
		if err := strings.TrimSpace(t.Error); err != "" {
			t.LastError = err
		}
		t.Status = StatusQueued
		t.Error = ""
		t.TaskID = ""
		t.Driver = ""
		t.StartedAt = 0
		t.FinishedAt = 0
		t.RetryCount++
		t.ProgressHint = "已 resume · 排队续跑"
	case StatusQueued:
		t.ProgressHint = "已 resume · 仍在排队"
	case StatusRunning:
		return Job{}, fmt.Errorf("job: goal still running")
	default:
		return Job{}, fmt.Errorf("job: cannot resume status %s", t.Status)
	}
	out := cloneJob(*t)
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return Job{}, err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return out, nil
}
