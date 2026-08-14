// Package toolgateway 是工具网关（Gateway）：注册 / 授权 / Invoke / MCP HTTP。
// 层职责：改世界只经本包；不定义生命周期。
// 工具分发走 Registry（RegisterHandler / ensureCoreHandlers），避免 CallNamedTool 巨型 switch。
// 本轮身份用 ProjectTurn / FinishTurn 与 lifecycle.RunState 同步（投影，非第二真相）。
// 壳层 UI 状态见 shell_ui.go；产品回调见 HostHooks（嵌入本结构以保持字段赋值兼容）。
package toolgateway

import (
	"fmt"
	"strings"
	"sync"

	nhjob "github.com/wcoreing/ningharness/job"
	session "github.com/wcoreing/ningharness/session"
	"github.com/wcoreing/ningharness/trace"
	"github.com/wcoreing/ningharness/workspace"
)

const ServerVersion = "0.1.6"

// Gateway 工具网关：注册 / 授权 / Invoke / MCP。
// 分区：HostHooks（壳回调）| ShellUI | 当前轮投影 | 工具 Registry。勿把 UI 当 Turn 真相。
type Gateway struct {
	HostHooks

	mu   sync.Mutex
	ws   *workspace.Service
	sess *session.Store

	ui ShellUI

	// --- 工具 Registry（CallNamedTool / Invoke；MCP 另绑 schema）---
	handlers       map[string]ToolHandler
	coreRegistered bool

	// --- 当前轮投影 + 授权策略（随 lifecycle 生灭）---
	turnPurpose    string
	turnAllow      map[string]bool
	turnTaskID     string
	turnSessionKey string
	turnParentTask string
	turnJobID      string
	taskTrace      *trace.Writer
	turnToolErrors []string

	Queue *nhjob.Manager
}

func New(ws *workspace.Service, sess *session.Store) *Gateway {
	if ws == nil {
		ws = workspace.New()
	}
	if sess == nil {
		sess = session.NewStore()
	}
	return &Gateway{ws: ws, sess: sess}
}

func (h *Gateway) Workspace() *workspace.Service { return h.ws }
func (h *Gateway) Session() *session.Store       { return h.sess }

func (h *Gateway) ArmTurnInterceptor(purpose string, allow map[string]bool) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.turnPurpose = strings.TrimSpace(purpose)
	h.turnAllow = allow
}

func (h *Gateway) DisarmTurnInterceptor() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.turnPurpose = ""
	h.turnAllow = nil
	h.turnTaskID = ""
	h.turnSessionKey = ""
	h.turnParentTask = ""
	h.turnJobID = ""
	h.turnToolErrors = nil
}

func (h *Gateway) SetTurnJobID(jobID string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.turnJobID = strings.TrimSpace(jobID)
}

func (h *Gateway) TurnJobID() string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.turnJobID
}

// SetTurnContext 兼容旧调用；保留已有 jobID，其余交给 ProjectTurn。
func (h *Gateway) SetTurnContext(taskID, sessionKey, parentTaskID string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	jobID := h.turnJobID
	h.mu.Unlock()
	h.ProjectTurn(taskID, sessionKey, parentTaskID, jobID)
}

func (h *Gateway) TurnContext() (taskID, sessionKey, parentTaskID string) {
	if h == nil {
		return "", "", ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.turnTaskID, h.turnSessionKey, h.turnParentTask
}

// SetTaskTrace 绑定当前 Task 的 JSONL Trace（可选；nil=关闭）。
func (h *Gateway) SetTaskTrace(w *trace.Writer) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.taskTrace = w
}

// TaskTrace 当前 Trace Writer。
func (h *Gateway) TaskTrace() *trace.Writer {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.taskTrace
}

func (h *Gateway) TurnPolicy() string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.turnPurpose
}

func (h *Gateway) CheckToolInterceptor(toolName string) error {
	return h.checkToolInterceptor(toolName)
}

func (h *Gateway) checkToolInterceptor(toolName string) error {
	h.mu.Lock()
	allow := h.turnAllow
	purpose := h.turnPurpose
	h.mu.Unlock()
	if allow == nil {
		return nil
	}
	name := strings.TrimSpace(toolName)
	if allow[name] {
		return nil
	}
	if purpose == "" {
		purpose = "restricted"
	}
	return fmt.Errorf("tool %s not allowed in this mode (purpose=%s)", name, purpose)
}

func (h *Gateway) pid() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if p := h.ws.Current(); p != nil {
		return p.ID
	}
	return ""
}

func (h *Gateway) activeSessionKey() string {
	root, err := h.root()
	if err != nil {
		return session.MainSessionID
	}
	if h.sess == nil {
		return session.MainSessionID
	}
	if f, err := h.sess.Snapshot(root, h.pid()); err == nil {
		if id := strings.TrimSpace(f.ActiveID); id != "" && !session.IsHiddenSession(id) {
			return id
		}
	}
	return session.MainSessionID
}

func (h *Gateway) AnnounceSidebarAssistant(text string) {
	text = strings.TrimSpace(text)
	if text == "" || h == nil || h.sess == nil {
		return
	}
	root, err := h.root()
	if err != nil {
		return
	}
	if _, err := h.sess.Append(root, h.pid(), h.activeSessionKey(), "assistant", text, "", ""); err != nil {
		return
	}
	if h.OnSessionChange != nil {
		h.OnSessionChange()
	}
}

func (h *Gateway) AnnounceQueueEnqueued(job nhjob.Job) {
	if !session.IsHiddenSession(job.SessionKey) {
		return
	}
	h.AnnounceSidebarAssistant(nhjob.FormatSidebarEnqueue(job))
}

func (h *Gateway) AnnounceQueueProgress(job nhjob.Job, kind nhjob.ProgressKind) {
	if !session.IsHiddenSession(job.SessionKey) {
		return
	}
	h.AnnounceSidebarAssistant(nhjob.FormatSidebarProgress(job, kind))
}

func (h *Gateway) pathExistsForAgent(rel string) bool {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" {
		return false
	}
	_, err := h.ws.ReadText(rel)
	return err == nil
}

// NoteToolError 记录本轮工具失败摘要（供 Memory 晋升 lesson）。
func (h *Gateway) NoteToolError(summary string) {
	if h == nil {
		return
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.turnToolErrors = append(h.turnToolErrors, summary)
}

// DrainToolErrors 取出并清空本轮工具失败摘要。
func (h *Gateway) DrainToolErrors() []string {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := append([]string(nil), h.turnToolErrors...)
	h.turnToolErrors = nil
	return out
}
