package job

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

// clearPauseLocked 解除暂停（入队 / 点继续）。
func (m *Manager) clearPauseLocked() {
	m.file.Paused = false
	m.file.PauseReason = ""
}
