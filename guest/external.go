package guest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RunState Values 键：Lifecycle run_guest 步据此选择 Guest 或 External。
const (
	ThinkValueKey = "think" // ThinkGuest | ThinkExternal
	ThinkGuest    = "guest"
	ThinkExternal = "external"

	ExternalModeKey         = "external_mode"
	ExternalAPIKeyKey       = "external_api_key"
	ExternalModelKey        = "external_model"
	ExternalSystemPromptKey = "external_system_prompt"
	ExternalRecentDialogKey = "external_recent_dialog"
)

// External 外置大脑（Cursor / Pi / dsh…）：怎么想在进程外，改世界仍须经 Gateway。
// 与 Guest.Run 并列：本地 Eino 用 Guest；外置进程用 External。
// Lifecycle 默认步骤仍名 run_guest：一步「思考」，内部按 ThinkValueKey 分支。
type External interface {
	Kind() string
	Ensure(ctx context.Context, onProgress func(stage, detail string)) error
	RunTurn(ctx context.Context, in ExternalTurn, tools Tools, emit Emit) (final string, err error)
	Abort(ctx context.Context, sessionKey string) error
	Close(ctx context.Context)
}

// ExternalTurn 一轮外置思考入参（会话/任务身份由宿主投影到 Gateway）。
type ExternalTurn struct {
	SessionKey     string
	TaskID         string
	Prompt         string
	Feedforward    string
	Mode           string // ask | agent | plan…
	APIKey         string
	Model          string
	SystemPrompt   string
	RecentDialog   string
	SkipUserAppend bool
	Root           string // 项目根（落 history / system）
}

// StateView Lifecycle RunState 的只读视图（避免 guest → lifecycle 依赖）。
type StateView struct {
	Root           string
	SessionKey     string
	TaskID         string
	Prompt         string
	Feedforward    string
	SkipUserAppend bool
	Values         map[string]any
}

// ExternalTurnFromState 从管道状态组装外置入参。
// 经 Lifecycle 时 AssembleContext 已落 user → SkipUserAppend 强制 true。
func ExternalTurnFromState(st StateView) ExternalTurn {
	in := ExternalTurn{
		SessionKey:     strings.TrimSpace(st.SessionKey),
		TaskID:         strings.TrimSpace(st.TaskID),
		Prompt:         st.Prompt,
		Feedforward:    st.Feedforward,
		Root:           strings.TrimSpace(st.Root),
		SkipUserAppend: true,
	}
	if st.Values == nil {
		return in
	}
	in.Mode = stringFromValues(st.Values, ExternalModeKey)
	in.APIKey = stringFromValues(st.Values, ExternalAPIKeyKey)
	in.Model = stringFromValues(st.Values, ExternalModelKey)
	in.SystemPrompt = stringFromValues(st.Values, ExternalSystemPromptKey)
	in.RecentDialog = stringFromValues(st.Values, ExternalRecentDialogKey)
	return in
}

// ThinkMode 本轮思考路径；空则 guest。
func ThinkMode(values map[string]any) string {
	s := strings.TrimSpace(stringFromValues(values, ThinkValueKey))
	if s == "" {
		return ThinkGuest
	}
	return s
}

func stringFromValues(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	v, ok := values[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return strings.TrimSpace(fmtSprint(v))
	}
	return strings.TrimSpace(s)
}

func fmtSprint(v any) string { return fmt.Sprint(v) }

// Emit 外置回合事件回调（宿主映射到 UI / Trace）。
type Emit func(Event)

// EventKind 外置事件种类。
type EventKind string

const (
	EventTextDelta EventKind = "text_delta"
	EventProgress  EventKind = "progress"
	EventToolStart EventKind = "tool_start"
	EventToolEnd   EventKind = "tool_end"
	EventError     EventKind = "error"
)

// Event 外置大脑 → 宿主。
type Event struct {
	Kind       EventKind
	Text       string
	Tool       string
	CallID     string
	Args       string
	Status     string
	Summary    string
	ResultJSON json.RawMessage
	ResourceID int64
	Stage      string
}

// ToolSpec 本轮可见工具定义（MCP / CustomTool schema 同构）。
type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
}

// InvokeResult 一次经 Gateway 的工具结果。
type InvokeResult struct {
	CallID     string
	OK         bool
	NeedsConfirm bool
	NeedsChoice  bool
	Error      string
	ConfirmSummary string
	Body       json.RawMessage
	ResourceID int64
}

// Tools 外置大脑共用的工具面：列表 + 经 Gateway 执行。
// 由宿主实现（可叠产品 Registry / 确认门 / caller）；框架只定契约。
type Tools interface {
	BindTurn(sessionKey, taskID string, ctx context.Context)
	ClearTurn()
	ListSpecs(mode string) []ToolSpec
	Invoke(ctx context.Context, name string, args json.RawMessage, callID string) (InvokeResult, error)
}
