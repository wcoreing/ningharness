package contract

// TreeNode 工作区树节点（与稿舍 WorkspaceTreeNode 契约对齐，便于后搬）。
type TreeNode struct {
	RelPath   string     `json:"relPath"`
	Name      string     `json:"name"`
	IsDir     bool       `json:"isDir"`
	WordCount int        `json:"wordCount,omitempty"` // 稿面字数（去 MD 壳）；目录为子合计
	Children  []TreeNode `json:"children,omitempty"`
}

// WorkspaceChangedEventName Wails EventsEmit 事件名。
const WorkspaceChangedEventName = "workspace-changed"

// ProjectOpenedEventName MCP / 外部打开项目后通知 UI 同步根路径。
const ProjectOpenedEventName = "project-opened"

// SessionChangedEventName 会话落盘后通知 UI 刷新 SessionBar。
const SessionChangedEventName = "session-changed"

// AgentEventName agentkit 流式事件（与落盘雷达分离；FE 可选订，勿塞进 protocol workspace store）。
const AgentEventName = "agent-event"

// TurnPipeStageEventName turnpipe 阶段推进（管道弹窗跟踪）。
const TurnPipeStageEventName = "turnpipe-stage"

// TurnPipeStageEvent 单阶段快照推进。
type TurnPipeStageEvent struct {
	TaskID string `json:"taskId"`
	Stage  string `json:"stage"`
	Brief  string `json:"brief,omitempty"`
	AtMs   int64  `json:"atMs"`
	OK     bool   `json:"ok"`
}

// GrowthChangedEventName Reflect 结束后「本轮学会了什么」摘要更新。
const GrowthChangedEventName = "growth-changed"

// QueueChangedEventName 队列快照变更（中栏队列面板 / 角标）。
const QueueChangedEventName = "queue-changed"

// ExportProgressEventName Word 导出进度（底栏）。
const ExportProgressEventName = "export-progress"

// SettingsChangedEventName 设置保存后通知 UI（刷新驱动列表等）。
const SettingsChangedEventName = "settings-changed"

// AgentkitReadyEventName Boot 完成后：驱动列表与默认值已就绪（FE 据此刷新对话栏，勿在 Boot 前缓存空列表）。
const AgentkitReadyEventName = "agentkit-ready"

// TermDataEventName / TermExitEventName 见 deskterm 包常量（FE 订阅同名）。

// ExportProgressEvent 导出进度载荷。
type ExportProgressEvent struct {
	Type    string `json:"type"` // status | done | error
	Step    string `json:"step,omitempty"`
	Message string `json:"message"`
	Percent int    `json:"percent,omitempty"`
}

// WorkspaceChangedEvent 落盘雷达事件。
type WorkspaceChangedEvent struct {
	ProjectID   string         `json:"projectId"`
	RelPaths    []string       `json:"relPaths,omitempty"`
	WriteID     string         `json:"writeId,omitempty"`
	WordCounts  map[string]int `json:"wordCounts,omitempty"` // rel → rune 字数（写盘瞬间推送，树可先涨）
}

// AgentDiffLine 写盘对照行。
type AgentDiffLine struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// AgentFileDiff 写盘 diff（Cursor 式预览）。
type AgentFileDiff struct {
	Path  string          `json:"path"`
	Add   int             `json:"add"`
	Del   int             `json:"del"`
	Lines []AgentDiffLine `json:"lines,omitempty"`
}

// AgentEvent FE 可见的 Agent 流事件载荷。
type AgentEvent struct {
	Type    string         `json:"type"`
	Message string         `json:"message,omitempty"`
	Text    string         `json:"text,omitempty"`
	Driver  string         `json:"driver,omitempty"`
	Phase   string         `json:"phase,omitempty"`  // tool: call | result
	CallID      string         `json:"callId,omitempty"` // 并行同名工具配对
	ResourceIDs []int64        `json:"resourceIds,omitempty"`
	Diff        *AgentFileDiff `json:"diff,omitempty"`
	JobID       string         `json:"jobId"`                  // 必填：队列 job id；非队列 once:{taskId} / desk:cancel
	TaskID      string         `json:"taskId,omitempty"`       // 执行台账 id（agenttask）
	SessionKey  string         `json:"sessionKey,omitempty"`   // 编排会话；侧栏按可见会话跟播
}

// Git 类型见 internal/gitdesk（直接绑定，避免双份结构体）。
