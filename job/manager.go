package job

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ningharness/goal"
)

// Executor 执行单条 agent-turn（同步；取消靠 ctx）。
type Executor func(ctx context.Context, job Job) (runID string, err error)

// ProgressHook 任务进度（节完成/终态）；在锁外调用，供侧栏话术等。
type ProgressHook func(job Job, kind ProgressKind)

// Manager 每项目一份队列；最多 MaxParallel 路并发执行 Executor。
type Manager struct {
	mu            sync.Mutex
	root          string
	file          File
	exec          Executor
	onChange    func(Snapshot)
	onProgress  ProgressHook
	maxParallel int
	running       map[string]context.CancelFunc // taskID → cancel
	kick          chan struct{}
	stop          chan struct{}
	stopped       bool
	loopStarted   bool
}

// DefaultMaxParallel 默认并发度（可 SetMaxParallel）。
const DefaultMaxParallel = 2

// New 创建管理器；exec 不可空。
func New(exec Executor, onChange func(Snapshot)) *Manager {
	if exec == nil {
		exec = func(context.Context, Job) (string, error) {
			return "", fmt.Errorf("job: no executor")
		}
	}
	m := &Manager{
		exec:        exec,
		onChange:    onChange,
		maxParallel: DefaultMaxParallel,
		running:     map[string]context.CancelFunc{},
		kick:        make(chan struct{}, 1),
		stop:        make(chan struct{}),
		file:        File{Version: 1, PauseOnError: true},
	}
	return m
}

// SetMaxParallel 设置并发上限；<1 则回落为 1；落盘到 queue.json。
func (m *Manager) SetMaxParallel(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n < 1 {
		n = 1
	}
	if n > 8 {
		n = 8
	}
	m.maxParallel = n
	m.file.MaxParallel = n
	if m.root != "" {
		_ = saveFile(m.root, m.file)
	}
	m.emitLocked()
	m.requestKickLocked()
}

// MaxParallel 当前并发上限。
func (m *Manager) MaxParallel() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.maxParallel
}

// SetProgressHook 设置进度回调（节完成 / 终态）。
func (m *Manager) SetProgressHook(h ProgressHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onProgress = h
}

// SetExecutor 绑定执行器（Open 后由宿主注入）。
func (m *Manager) SetExecutor(exec Executor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exec == nil {
		exec = func(context.Context, Job) (string, error) {
			return "", fmt.Errorf("job: no executor")
		}
	}
	m.exec = exec
}

// SetOnChange 绑定队列快照变更回调。
func (m *Manager) SetOnChange(fn func(Snapshot)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}


func (m *Manager) fireProgress(job Job, kind ProgressKind) {
	m.mu.Lock()
	h := m.onProgress
	m.mu.Unlock()
	if h != nil {
		h(job, kind)
	}
}

func cloneJob(j Job) Job {
	if len(j.Steps) == 0 {
		return j
	}
	steps := make([]Step, len(j.Steps))
	copy(steps, j.Steps)
	j.Steps = steps
	return j
}

func clampParallel(n int) int {
	if n < 1 {
		return DefaultMaxParallel
	}
	if n > 8 {
		return 8
	}
	return n
}

// Bind 切换项目根并加载队列；空 root 清空内存态。
func (m *Manager) Bind(root string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	root = strings.TrimSpace(root)
	if m.root == root && root != "" {
		return
	}
	if len(m.running) > 0 {
		for id, cancel := range m.running {
			cancel()
			delete(m.running, id)
		}
	}
	m.root = root
	if root == "" {
		m.file = File{Version: 1, PauseOnError: true}
		m.maxParallel = DefaultMaxParallel
		m.emitLocked()
		return
	}
	f, err := loadFile(root)
	if err != nil {
		m.file = File{Version: 1, PauseOnError: true}
	} else {
		m.file = f
	}
	m.maxParallel = clampParallel(m.file.MaxParallel)
	m.file.MaxParallel = m.maxParallel
	_ = saveFile(root, m.file) // 固化「残留 running → error」
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
}

// Shutdown 停止 worker（App 退出）。
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return
	}
	m.stopped = true
	close(m.stop)
	for id, cancel := range m.running {
		cancel()
		delete(m.running, id)
	}
}

func (m *Manager) ensureLoopLocked() {
	if m.loopStarted || m.stopped {
		return
	}
	m.loopStarted = true
	go m.loop()
}

func (m *Manager) requestKickLocked() {
	select {
	case m.kick <- struct{}{}:
	default:
	}
}

// Snapshot 当前快照。
func (m *Manager) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapLocked()
}

func (m *Manager) snapLocked() Snapshot {
	tasks := append([]Job(nil), m.file.Jobs...)
	return Snapshot{
		Paused:       m.file.Paused,
		PauseOnError: m.file.PauseOnError,
		PauseReason:  m.file.PauseReason,
		MaxParallel:  m.maxParallel,
		Jobs:        tasks,
		Stats:        computeStats(tasks),
	}
}

// HasRunning 是否有执行中任务。
func (m *Manager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.running) > 0 || computeStats(m.file.Jobs).Running > 0
}

// RunningIDs 当前内存中执行中的 task id（取消前快照用）。
func (m *Manager) RunningIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.running))
	for id := range m.running {
		out = append(out, id)
	}
	return out
}

// Brief Turn Transport 一行摘要。
func (m *Manager) Brief() string {
	s := m.Snapshot()
	if s.Stats.Queued == 0 && s.Stats.Running == 0 && !s.Paused {
		return ""
	}
	if s.Stats.Queued == 0 && s.Stats.Running == 0 && s.Paused {
		return "队列：已暂停（无在途任务）"
	}
	pause := ""
	if s.Paused {
		pause = " · 已暂停"
		if r := strings.TrimSpace(s.PauseReason); r != "" {
			pause += "：" + trimReason(r, 48)
		}
	}
	mp := s.MaxParallel
	if mp < 1 {
		mp = DefaultMaxParallel
	}
	return fmt.Sprintf("队列：running=%d queued=%d ·并行≤%d%s", s.Stats.Running, s.Stats.Queued, mp, pause)
}

func trimReason(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if max <= 0 || len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// clearPauseLocked 解除暂停（入队 / 点继续）。
func (m *Manager) clearPauseLocked() {
	m.file.Paused = false
	m.file.PauseReason = ""
}

// Enqueue 入队一条 agent-turn（sessionKey 空则执行时用侧栏 active）。
// 侧栏发送请用 App.QueueEnqueue / EnqueueSession 钉住 active。
// MCP enqueue_agent_turn 默认 EnqueueSession(active)；仅 session=isolated 时用 EnqueueIsolated。
func (m *Manager) Enqueue(prompt, driver, title, targetRel string) (Job, error) {
	return m.EnqueueSession(prompt, driver, title, targetRel, "", "", "")
}

// EnqueueIsolated 隐藏会话 once:queue:{id}：与侧栏 main 及并行任务隔离（MCP session=isolated / EnqueuePaths 空 session）。
func (m *Manager) EnqueueIsolated(prompt, driver, title, targetRel string) (Job, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return Job{}, fmt.Errorf("job: prompt empty")
	}
	targetRel = strings.TrimSpace(strings.ReplaceAll(targetRel, "\\", "/"))
	targetRel = strings.TrimPrefix(targetRel, "./")
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.TrimSpace(m.root) == "" {
		return Job{}, fmt.Errorf("job: no project")
	}
	now := time.Now().UnixMilli()
	id := newID("q")
	t := Job{
		ID:         id,
		Type:       JobTypeAgentTurn,
		Title:      strings.TrimSpace(title),
		Prompt:     prompt,
		Driver:     strings.TrimSpace(driver),
		TargetRel:  targetRel,
		Status:     StatusQueued,
		CreatedAt:  now,
		SessionKey: QueueSessionKey(id),
	}
	if t.Title == "" {
		t.Title = titleFromPrompt(prompt, t.TargetRel)
	}
	m.file.Jobs = append(m.file.Jobs, t)
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return Job{}, err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return t, nil
}

// EnqueueSession 入队并指定编排会话与 Purpose（ask=只读；空=Agent）。model 入队时钉死。
// sessionKey 空：执行时回落侧栏 active（仅兼容旧调用；新 UI 应入队时钉死）。
func (m *Manager) EnqueueSession(prompt, driver, title, targetRel, sessionKey, purpose, model string) (Job, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return Job{}, fmt.Errorf("job: prompt empty")
	}
	targetRel = strings.TrimSpace(strings.ReplaceAll(targetRel, "\\", "/"))
	targetRel = strings.TrimPrefix(targetRel, "./")
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.TrimSpace(m.root) == "" {
		return Job{}, fmt.Errorf("job: no project")
	}
	now := time.Now().UnixMilli()
	t := Job{
		ID:         newID("q"),
		Type:       JobTypeAgentTurn,
		Title:      strings.TrimSpace(title),
		Prompt:     prompt,
		Driver:     strings.TrimSpace(driver),
		Model:      strings.TrimSpace(model),
		TargetRel:  targetRel,
		Status:     StatusQueued,
		CreatedAt:  now,
		SessionKey: strings.TrimSpace(sessionKey),
		Purpose:    strings.TrimSpace(purpose),
	}
	if t.Title == "" {
		t.Title = titleFromPrompt(prompt, t.TargetRel)
	}
	m.file.Jobs = append(m.file.Jobs, t)
	// 用户主动入队 = 继续干活（失败后暂停常被误当成「卡死」）
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return Job{}, err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return t, nil
}

// EnqueueGoal 入队 Goal 外环：Prompt=objective；反复 Executor 直到 GOAL.yaml 终态或超轮。
// sessionKey 空则 once:queue:{id}。maxRounds<1 则 DefaultGoalMaxRounds。
func (m *Manager) EnqueueGoal(objective, driver, title, sessionKey, purpose, model string, maxRounds int) (Job, error) {
	objective = strings.TrimSpace(objective)
	if objective == "" {
		return Job{}, fmt.Errorf("job: objective empty")
	}
	if maxRounds < 1 {
		maxRounds = DefaultGoalMaxRounds
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.TrimSpace(m.root) == "" {
		return Job{}, fmt.Errorf("job: no project")
	}
	now := time.Now().UnixMilli()
	id := newID("q")
	sk := strings.TrimSpace(sessionKey)
	if sk == "" {
		sk = QueueSessionKey(id)
	}
	t := Job{
		ID:            id,
		Type:          JobTypeGoal,
		Title:         strings.TrimSpace(title),
		Prompt:        objective,
		Driver:        strings.TrimSpace(driver),
		Model:         strings.TrimSpace(model),
		Status:        StatusQueued,
		CreatedAt:     now,
		SessionKey:    sk,
		Purpose:       strings.TrimSpace(purpose),
		GoalMaxRounds: maxRounds,
	}
	if t.Title == "" {
		t.Title = titleFromPrompt(objective, "")
	}
	m.file.Jobs = append(m.file.Jobs, t)
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return Job{}, err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return t, nil
}

// EnqueuePaths 按路径批量入队：一条任务内串行多节。
// sessionKey 空则 once:queue:{id}（批处理隔离；对话入队请用 EnqueueSession/active）。
// 侧栏勾选入队应传入 activeSessionId 以续接对话记忆。
func (m *Manager) EnqueuePaths(paths []string, promptTpl, driver, sessionKey, model string) ([]Job, error) {
	clean := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, p := range paths {
		r := strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
		r = strings.TrimPrefix(r, "./")
		if r == "" || strings.Contains(r, "..") {
			continue
		}
		if seen[r] {
			continue
		}
		seen[r] = true
		clean = append(clean, r)
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("job: no valid paths")
	}
	if len(clean) > maxEnqueuePaths {
		return nil, fmt.Errorf("job: too many paths (max %d)", maxEnqueuePaths)
	}
	tpl := strings.TrimSpace(promptTpl)
	if tpl == "" {
		tpl = DefaultPathPrompt
	}
	driver = strings.TrimSpace(driver)
	model = strings.TrimSpace(model)

	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.TrimSpace(m.root) == "" {
		return nil, fmt.Errorf("job: no project")
	}
	now := time.Now().UnixMilli()
	id := newID("q")
	steps := make([]Step, 0, len(clean))
	for _, rel := range clean {
		steps = append(steps, Step{
			Rel:    rel,
			Title:  filepath.Base(rel),
			Status: StepPending,
		})
	}
	title := filepath.Base(clean[0])
	if len(clean) > 1 {
		title = fmt.Sprintf("%s 等 %d 个文件", filepath.Base(clean[0]), len(clean))
	}
	sk := strings.TrimSpace(sessionKey)
	if sk == "" {
		sk = QueueSessionKey(id)
	}
	t := Job{
		ID:           id,
		Type:         JobTypeAgentTurn,
		Title:        title,
		Prompt:       tpl,
		Driver:       driver,
		Model:        model,
		TargetRel:    clean[0],
		Status:       StatusQueued,
		CreatedAt:    now,
		SessionKey:   sk,
		Steps:        steps,
		StepDone:     0,
		StepTotal:    len(steps),
		ProgressHint: fmt.Sprintf("排队 · 0/%d", len(steps)),
	}
	m.file.Jobs = append(m.file.Jobs, t)
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return nil, err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return []Job{t}, nil
}

// EnqueueSteps 入队一条多节任务；每节可自带 Prompt（覆盖 Job.Prompt）。
// feedExtra 固化本轮评分/路径等前馈块（执行时并入 feedforward）。
func (m *Manager) EnqueueSteps(title, driver, sessionKey, purpose string, steps []Step, feedExtra string) (Job, error) {
	if len(steps) == 0 {
		return Job{}, fmt.Errorf("job: empty steps")
	}
	clean := make([]Step, 0, len(steps))
	for _, s := range steps {
		rel := strings.TrimSpace(strings.ReplaceAll(s.Rel, "\\", "/"))
		rel = strings.TrimPrefix(rel, "./")
		prompt := strings.TrimSpace(s.Prompt)
		if rel == "" && prompt == "" {
			continue
		}
		if rel == "" {
			rel = fmt.Sprintf("step-%d", len(clean)+1)
		}
		title := strings.TrimSpace(s.Title)
		if title == "" {
			title = filepath.Base(rel)
		}
		clean = append(clean, Step{
			Rel:    rel,
			Title:  title,
			Prompt: prompt,
			Status: StepPending,
		})
	}
	if len(clean) == 0 {
		return Job{}, fmt.Errorf("job: empty steps")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.TrimSpace(m.root) == "" {
		return Job{}, fmt.Errorf("job: no project")
	}
	now := time.Now().UnixMilli()
	id := newID("q")
	sk := strings.TrimSpace(sessionKey)
	if sk == "" {
		sk = QueueSessionKey(id)
	}
	jobTitle := strings.TrimSpace(title)
	if jobTitle == "" {
		jobTitle = clean[0].Title
	}
	// Job.Prompt 作兜底；各节优先用 Step.Prompt
	fallback := strings.TrimSpace(clean[0].Prompt)
	if fallback == "" {
		fallback = DefaultPathPrompt
	}
	t := Job{
		ID:           id,
		Type:         JobTypeAgentTurn,
		Title:        jobTitle,
		Prompt:       fallback,
		Driver:       strings.TrimSpace(driver),
		TargetRel:    clean[0].Rel,
		Status:       StatusQueued,
		CreatedAt:    now,
		SessionKey:   sk,
		Purpose:      strings.TrimSpace(purpose),
		FeedExtra:    strings.TrimSpace(feedExtra),
		Steps:        clean,
		StepDone:     0,
		StepTotal:    len(clean),
		ProgressHint: fmt.Sprintf("排队 · 0/%d", len(clean)),
	}
	m.file.Jobs = append(m.file.Jobs, t)
	m.clearPauseLocked()
	if err := saveFile(m.root, m.file); err != nil {
		return Job{}, err
	}
	m.ensureLoopLocked()
	m.emitLocked()
	m.requestKickLocked()
	return t, nil
}

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

func goalControlPath(root, jobID string) string {
	return filepath.Join(root, ".ningharness", "goals", jobID, "GOAL.yaml")
}

func (m *Manager) runGoal(ctx context.Context, job Job) {
	root := m.root
	control := goalControlPath(root, job.ID)
	planRel := filepath.ToSlash(filepath.Join(".ningharness", "goals", job.ID, "PLAN.md"))
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
				t.ProgressHint = fmt.Sprintf("目标受阻 · 第 %d 轮", t.GoalRound)
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
	if kind != "" {
		m.fireProgress(done, kind)
	}
}

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

func newID(prefix string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixMilli(), hex.EncodeToString(b[:]))
}

func titleFromPrompt(prompt, targetRel string) string {
	if targetRel != "" {
		return filepath.Base(targetRel)
	}
	r := []rune(strings.TrimSpace(prompt))
	if len(r) > 36 {
		return string(r[:36]) + "…"
	}
	if len(r) == 0 {
		return "agent-turn"
	}
	return string(r)
}
