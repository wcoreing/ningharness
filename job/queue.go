package job

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// CancelRunning 取消全部执行中任务。
func (m *Manager) CancelRunning() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.running))
	for id := range m.running {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Cancel(id)
	}
}

// Cancel 取消排队或打断执行中。
func (m *Manager) Cancel(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("job: empty task id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == "" {
		return fmt.Errorf("job: no project")
	}
	now := time.Now().UnixMilli()
	found := false
	for i := range m.file.Jobs {
		t := &m.file.Jobs[i]
		if t.ID != taskID {
			continue
		}
		found = true
		switch t.Status {
		case StatusQueued:
			t.Status = StatusCancelled
			t.FinishedAt = now
			m.flushOrphanSteerLocked(t)
		case StatusRunning:
			if cancel, ok := m.running[taskID]; ok {
				cancel()
				delete(m.running, taskID)
			}
			t.Status = StatusCancelled
			t.FinishedAt = now
			t.Error = "cancelled"
			m.flushOrphanSteerLocked(t)
		default:
			return fmt.Errorf("job: task not cancellable (%s)", t.Status)
		}
		break
	}
	if !found {
		return fmt.Errorf("job: task not found")
	}
	if err := saveFile(m.root, m.file); err != nil {
		return err
	}
	m.emitLocked()
	m.requestKickLocked()
	return nil
}

// SetPaused 暂停/继续调度。
func (m *Manager) SetPaused(paused bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == "" {
		return fmt.Errorf("job: no project")
	}
	m.file.Paused = paused
	if !paused {
		m.file.PauseReason = ""
	} else if strings.TrimSpace(m.file.PauseReason) == "" {
		m.file.PauseReason = "手动暂停"
	}
	if err := saveFile(m.root, m.file); err != nil {
		return err
	}
	m.emitLocked()
	if !paused {
		m.requestKickLocked()
	}
	return nil
}

// SetPauseOnError 失败后是否自动暂停。
func (m *Manager) SetPauseOnError(v bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == "" {
		return fmt.Errorf("job: no project")
	}
	m.file.PauseOnError = v
	if err := saveFile(m.root, m.file); err != nil {
		return err
	}
	m.emitLocked()
	return nil
}

// MoveBefore 将排队中任务移到 beforeID 之前（beforeID 空=排到 queued 队尾）。
// 仅 StatusQueued 可调序；running/终态不动。
func (m *Manager) MoveBefore(taskID, beforeID string) error {
	taskID = strings.TrimSpace(taskID)
	beforeID = strings.TrimSpace(beforeID)
	if taskID == "" {
		return fmt.Errorf("job: empty task id")
	}
	if beforeID == taskID {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == "" {
		return fmt.Errorf("job: no project")
	}
	var queued []Job
	fromIdx := -1
	for _, t := range m.file.Jobs {
		if t.Status == StatusQueued {
			if t.ID == taskID {
				fromIdx = len(queued)
			}
			queued = append(queued, t)
		}
	}
	if fromIdx < 0 {
		return fmt.Errorf("job: task not queued (only queued can reorder)")
	}
	item := queued[fromIdx]
	queued = append(queued[:fromIdx], queued[fromIdx+1:]...)
	insertAt := len(queued)
	if beforeID != "" {
		insertAt = -1
		for i, t := range queued {
			if t.ID == beforeID {
				insertAt = i
				break
			}
		}
		if insertAt < 0 {
			return fmt.Errorf("job: before task not found or not queued")
		}
	}
	next := make([]Job, 0, len(queued)+1)
	next = append(next, queued[:insertAt]...)
	next = append(next, item)
	next = append(next, queued[insertAt:]...)
	// 保持非 queued 相对顺序，queued 块按其创建先后夹在中间：重建为 rest 中非 queued + next queued
	// 实际落盘顺序：终态/running 保持原相对位置，queued 作为连续块按新顺序插回「第一个原 queued 位置」。
	m.file.Jobs = rebuildWithQueuedOrder(m.file.Jobs, next)
	if err := saveFile(m.root, m.file); err != nil {
		return err
	}
	m.emitLocked()
	return nil
}

// MoveByDir 将排队任务上移/下移一格（dir=up|down）。
func (m *Manager) MoveByDir(taskID, dir string) error {
	dir = strings.ToLower(strings.TrimSpace(dir))
	taskID = strings.TrimSpace(taskID)
	m.mu.Lock()
	var queuedIDs []string
	idx := -1
	for _, t := range m.file.Jobs {
		if t.Status != StatusQueued {
			continue
		}
		if t.ID == taskID {
			idx = len(queuedIDs)
		}
		queuedIDs = append(queuedIDs, t.ID)
	}
	m.mu.Unlock()
	if idx < 0 {
		return fmt.Errorf("job: task not queued")
	}
	switch dir {
	case "up":
		if idx == 0 {
			return nil
		}
		return m.MoveBefore(taskID, queuedIDs[idx-1])
	case "down":
		if idx >= len(queuedIDs)-1 {
			return nil
		}
		// 移到下一项之后 = 移到下下项之前；若是倒数第二则 before 空
		if idx+2 < len(queuedIDs) {
			return m.MoveBefore(taskID, queuedIDs[idx+2])
		}
		return m.MoveBefore(taskID, "")
	default:
		return fmt.Errorf("job: dir must be up|down")
	}
}

func rebuildWithQueuedOrder(all []Job, queuedOrdered []Job) []Job {
	out := make([]Job, 0, len(all))
	inserted := false
	for _, t := range all {
		if t.Status == StatusQueued {
			if !inserted {
				out = append(out, queuedOrdered...)
				inserted = true
			}
			continue
		}
		out = append(out, t)
	}
	if !inserted {
		out = append(out, queuedOrdered...)
	}
	return out
}

// Start 单任务运行：失败/已取消重排队；已在排队则开闸调度。对齐稿舍「启动」。
func (m *Manager) Start(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("job: empty task id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == "" {
		return fmt.Errorf("job: no project")
	}
	idx := -1
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID == taskID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("job: task not found")
	}
	t := &m.file.Jobs[idx]
	switch t.Status {
	case StatusError, StatusCancelled, StatusDone:
		if strings.TrimSpace(t.Prompt) == "" {
			return fmt.Errorf("job: empty prompt")
		}
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
		resetIncompleteSteps(t)
	case StatusQueued:
		// 已在排队：仅开闸
	default:
		return fmt.Errorf("job: only queued/failed/cancelled/done can start (%s)", t.Status)
	}
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return nil
}

// Delete 删除任务：若执行中先取消，再从队列移除。
func (m *Manager) Delete(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("job: empty task id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == "" {
		return fmt.Errorf("job: no project")
	}
	idx := -1
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID == taskID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("job: task not found")
	}
	t := m.file.Jobs[idx]
	if t.Status == StatusRunning {
		if cancel, ok := m.running[taskID]; ok {
			cancel()
			delete(m.running, taskID)
		}
	}
	m.file.Jobs = append(m.file.Jobs[:idx], m.file.Jobs[idx+1:]...)
	if err := saveFile(m.root, m.file); err != nil {
		return err
	}
	m.emitLocked()
	m.requestKickLocked()
	return nil
}

// Retry 将失败/已取消任务就地改回排队（保留同一 task id / prompt / 目标），并标记失败重启。
func (m *Manager) Retry(taskID string) (Job, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return Job{}, fmt.Errorf("job: empty task id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == "" {
		return Job{}, fmt.Errorf("job: no project")
	}
	idx := -1
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID == taskID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Job{}, fmt.Errorf("job: task not found")
	}
	t := &m.file.Jobs[idx]
	if t.Status != StatusError && t.Status != StatusCancelled {
		return Job{}, fmt.Errorf("job: only failed/cancelled can retry (%s)", t.Status)
	}
	if strings.TrimSpace(t.Prompt) == "" {
		return Job{}, fmt.Errorf("job: empty prompt")
	}
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
	resetIncompleteSteps(t)
	out := *t
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return Job{}, err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return out, nil
}

// RetryFailed 将全部失败任务就地改回排队（失败重启）；返回重启条数。
func (m *Manager) RetryFailed() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == "" {
		return 0, fmt.Errorf("job: no project")
	}
	n := m.requeueFailedLocked(false)
	if n == 0 {
		return 0, fmt.Errorf("job: no failed tasks to retry")
	}
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return 0, err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return n, nil
}

// requeueFailedLocked 将 error（及 optionally cancelled）改回 queued；调用方持锁。
func (m *Manager) requeueFailedLocked(includeCancelled bool) int {
	n := 0
	for i := range m.file.Jobs {
		t := &m.file.Jobs[i]
		switch t.Status {
		case StatusError:
		case StatusCancelled:
			if !includeCancelled {
				continue
			}
		default:
			continue
		}
		if strings.TrimSpace(t.Prompt) == "" {
			continue
		}
		if err := strings.TrimSpace(t.Error); err != "" {
			t.LastError = err
		}
		t.Status = StatusQueued
		t.Error = ""
		t.TaskID = ""
		// 重跑不沿用历史钉死的 driver（如旧 cursor），交给运行时默认驱动
		t.Driver = ""
		t.StartedAt = 0
		t.FinishedAt = 0
		t.RetryCount++
		resetIncompleteSteps(t)
		n++
	}
	return n
}

// Run 唯一「运行」：失败/已取消重回排队，开闸并调度。已有排队项时即使无失败也可点。
func (m *Manager) Run() (requeued int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root == "" {
		return 0, fmt.Errorf("job: no project")
	}
	requeued = m.requeueFailedLocked(true)
	stats := computeStats(m.file.Jobs)
	if requeued == 0 && stats.Queued == 0 && stats.Running == 0 {
		return 0, fmt.Errorf("job: nothing to run")
	}
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return 0, err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return requeued, nil
}

// Stop 唯一「停止」：关闸并取消执行中任务。
func (m *Manager) Stop() error {
	m.mu.Lock()
	if m.root == "" {
		m.mu.Unlock()
		return fmt.Errorf("job: no project")
	}
	m.file.Paused = true
	m.file.PauseReason = "已停止"
	if err := saveFile(m.root, m.file); err != nil {
		m.mu.Unlock()
		return err
	}
	m.emitLocked()
	m.mu.Unlock()
	m.CancelRunning()
	return nil
}

func (m *Manager) emitLocked() {
	if m.onChange == nil {
		return
	}
	snap := m.snapLocked()
	go m.onChange(snap)
}

func (m *Manager) loop() {
	for {
		select {
		case <-m.stop:
			return
		case <-m.kick:
			m.dispatchOnce()
		}
	}
}

func (m *Manager) dispatchOnce() {
	for {
		task, ctx, ok := m.tryStartOne()
		if !ok {
			return
		}
		go m.runOne(ctx, task)
	}
}

func (m *Manager) tryStartOne() (Job, context.Context, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped || m.root == "" || m.file.Paused {
		return Job{}, nil, false
	}
	if len(m.running) >= m.maxParallel {
		return Job{}, nil, false
	}
	idx := -1
	for i := range m.file.Jobs {
		if m.file.Jobs[i].Status == StatusQueued {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Job{}, nil, false
	}
	job := m.file.Jobs[idx]
	job.Status = StatusRunning
	job.StartedAt = time.Now().UnixMilli()
	m.file.Jobs[idx] = job
	ctx, cancel := context.WithCancel(context.Background())
	m.running[job.ID] = cancel
	_ = saveFile(m.root, m.file)
	m.emitLocked()
	return job, ctx, true
}

func (m *Manager) runOne(ctx context.Context, job Job) {
	if job.Type == JobTypeGoal {
		m.runGoal(ctx, job)
		return
	}
	if len(job.Steps) > 0 {
		m.runSteps(ctx, job)
		return
	}
	runID, err := m.exec(ctx, job)

	m.mu.Lock()
	var done Job
	var kind ProgressKind
	delete(m.running, job.ID)
	// 任务可能已被 Cancel 改成 cancelled
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID != job.ID {
			continue
		}
		t := &m.file.Jobs[i]
		if t.Status == StatusCancelled {
			break
		}
		t.TaskID = strings.TrimSpace(runID)
		t.FinishedAt = time.Now().UnixMilli()
		if ctx.Err() != nil {
			t.Status = StatusCancelled
			t.Error = "cancelled"
			kind = ProgressCancelled
		} else if err != nil {
			t.Status = StatusError
			t.Error = err.Error()
			t.LastError = t.Error
			kind = ProgressError
			if m.file.PauseOnError {
				m.file.Paused = true
				m.file.PauseReason = "上一任务失败：" + trimReason(err.Error(), 120)
			}
		} else {
			t.Status = StatusDone
			t.Error = ""
			kind = ProgressDone
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
