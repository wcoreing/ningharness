package job

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

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
func (m *Manager) EnqueueGoal(in GoalEnqueue) (Job, error) {
	objective := strings.TrimSpace(in.Objective)
	if objective == "" {
		return Job{}, fmt.Errorf("job: objective empty")
	}
	maxRounds := in.MaxRounds
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
	sk := strings.TrimSpace(in.SessionKey)
	if sk == "" {
		sk = QueueSessionKey(id)
	}
	t := Job{
		ID:            id,
		Type:          JobTypeGoal,
		Title:         strings.TrimSpace(in.Title),
		Prompt:        objective,
		Driver:        strings.TrimSpace(in.Driver),
		Model:         strings.TrimSpace(in.Model),
		Status:        StatusQueued,
		CreatedAt:     now,
		SessionKey:    sk,
		Purpose:       strings.TrimSpace(in.Purpose),
		FeedExtra:     strings.TrimSpace(in.FeedExtra),
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
