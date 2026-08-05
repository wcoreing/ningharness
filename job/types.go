// Package job Agent 队列：Job = 调度单元；执行台账见 ningharness/task。
package job

import "strings"

// Status 队列 Job 可见状态。
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusError     Status = "error"
	StatusCancelled Status = "cancelled"
)

// StepStatus 批内节状态。
type StepStatus string

const (
	StepPending StepStatus = "pending"
	StepRunning StepStatus = "running"
	StepDone    StepStatus = "done"
	StepError   StepStatus = "error"
)

// JobTypeAgentTurn 一条 prompt → 一轮或多轮串行 agentkit.Run。
const JobTypeAgentTurn = "agent-turn"

// JobTypeGoal 外环：反复跑 Executor，直到 GOAL.yaml status 终态或超轮。
const JobTypeGoal = "goal"

// DefaultGoalMaxRounds Goal 外环默认硬上限。
const DefaultGoalMaxRounds = 100

// DefaultPathPrompt MCP/空模板时的兜底。
const DefaultPathPrompt = `请按任务说明处理本节。完成后聊天只回路径与短确认，不要贴全文。`

const (
	relStore        = ".agentdesk/queue.json"
	maxFinishedKeep = 80
	maxEnqueuePaths = 50
)

// Step 批 Job 内的一节；串行执行、共用 SessionKey。
type Step struct {
	Rel    string     `json:"rel"`
	Title  string     `json:"title,omitempty"`
	Prompt string     `json:"prompt,omitempty"` // 非空则覆盖 Job.Prompt（风格训练等）
	Status StepStatus `json:"status"`
	TaskID string     `json:"taskId,omitempty"` // 执行台账 id（agenttask）
	Error  string     `json:"error,omitempty"`
}

// Job 队列调度单元（可含多 step）。
type Job struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Prompt     string `json:"prompt"`
	WirePrompt string `json:"-"`
	Driver     string `json:"driver,omitempty"`
	Model      string `json:"model,omitempty"`
	TargetRel  string `json:"targetRel,omitempty"`
	Status     Status `json:"status"`
	TaskID     string `json:"taskId,omitempty"` // 最近一次执行台账
	Error      string `json:"error,omitempty"`
	LastError  string `json:"lastError,omitempty"`
	RetryCount int    `json:"retryCount,omitempty"`
	BatchID    string `json:"batchId,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	StartedAt  int64  `json:"startedAt,omitempty"`
	FinishedAt int64  `json:"finishedAt,omitempty"`

	SessionKey string `json:"sessionKey,omitempty"`
	Purpose    string `json:"purpose,omitempty"`
	// FeedExtra 入队时固化的前馈块（如练笔评分/路径）；执行时并入 history_message.feedforward。
	FeedExtra  string `json:"feedExtra,omitempty"`
	Steps      []Step `json:"steps,omitempty"`
	StepDone   int    `json:"stepDone,omitempty"`
	StepTotal  int    `json:"stepTotal,omitempty"`
	ProgressHint string `json:"progressHint,omitempty"`

	GoalMaxRounds int `json:"goalMaxRounds,omitempty"`
	GoalRound     int `json:"goalRound,omitempty"`
	// SteerPending 运行中插话（人引导）；下一工具结果或下一 Goal 轮注入后清空。
	SteerPending string `json:"steerPending,omitempty"`
}

// File 落盘格式。
type File struct {
	Version      int    `json:"version"`
	Paused       bool   `json:"paused"`
	PauseOnError bool   `json:"pauseOnError"`
	PauseReason  string `json:"pauseReason,omitempty"`
	MaxParallel  int    `json:"maxParallel,omitempty"`
	Jobs         []Job  `json:"jobs"`
	// LegacyTasks 仅迁移读旧 queue.json 的 tasks 字段。
	LegacyTasks []Job `json:"tasks,omitempty"`
	History     []Job `json:"history,omitempty"`
}

// Stats 聚合。
type Stats struct {
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Done      int `json:"done"`
	Error     int `json:"error"`
	Cancelled int `json:"cancelled"`
}

// Snapshot UI / MCP 可见快照。
type Snapshot struct {
	Paused       bool   `json:"paused"`
	PauseOnError bool   `json:"pauseOnError"`
	PauseReason  string `json:"pauseReason,omitempty"`
	MaxParallel  int    `json:"maxParallel"`
	Jobs         []Job  `json:"jobs"`
	Stats        Stats  `json:"stats"`
}

// QueueSessionKey 批/写手编排键。
func QueueSessionKey(jobID string) string {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return "once:queue"
	}
	return "once:queue:" + jobID
}
