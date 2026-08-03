package hub

import (
	"fmt"
	"strings"
	"sync"

	deskqueue "ningharness/job"
	session "ningharness/session"
	"ningharness/workspace"
)

const ServerVersion = "0.1.0"

type Hub struct {
	mu            sync.Mutex
	ws            *workspace.Service
	sess          *session.Store
	focusRel      string
	contentTabs   string
	treeSelection string
	turnPurpose   string
	turnAllow     map[string]bool
	turnTaskID    string
	turnSessionKey string
	turnParentTask string
	Queue         *deskqueue.Manager
	OnOpen        func(root string)
	OnSessionChange func()
	OnPathsChanged func(writeID string, paths []string, wordCounts map[string]int)
	OnPathsMoved   func(movedTo map[string]string)
	OnWriteWorktree func(relPath, content, writeID string) error
	OnWriteBytes  func(relPath string, data []byte, writeID string) error
}

func NewHub(ws *workspace.Service, sess *session.Store) *Hub {
	if ws == nil {
		ws = workspace.New()
	}
	if sess == nil {
		sess = session.NewStore()
	}
	return &Hub{ws: ws, sess: sess}
}

func (h *Hub) Workspace() *workspace.Service { return h.ws }
func (h *Hub) Session() *session.Store       { return h.sess }

func (h *Hub) SetFocus(relPath string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.focusRel = strings.TrimSpace(relPath)
}

func (h *Hub) Focus() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.focusRel
}

func (h *Hub) SetContentTabsBrief(brief string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.contentTabs = strings.TrimSpace(brief)
}

func (h *Hub) ContentTabsBrief() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.contentTabs
}

func (h *Hub) SetTreeSelectionBrief(brief string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.treeSelection = strings.TrimSpace(brief)
}

func (h *Hub) TreeSelectionBrief() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.treeSelection
}

func (h *Hub) ArmTurnInterceptor(purpose string, allow map[string]bool) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.turnPurpose = strings.TrimSpace(purpose)
	h.turnAllow = allow
}

func (h *Hub) DisarmTurnInterceptor() {
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
}

func (h *Hub) SetTurnContext(taskID, sessionKey, parentTaskID string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.turnTaskID = strings.TrimSpace(taskID)
	h.turnSessionKey = strings.TrimSpace(sessionKey)
	h.turnParentTask = strings.TrimSpace(parentTaskID)
}

func (h *Hub) TurnContext() (taskID, sessionKey, parentTaskID string) {
	if h == nil {
		return "", "", ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.turnTaskID, h.turnSessionKey, h.turnParentTask
}

func (h *Hub) TurnPolicy() string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.turnPurpose
}

func (h *Hub) CheckToolInterceptor(toolName string) error {
	return h.checkToolInterceptor(toolName)
}

func (h *Hub) checkToolInterceptor(toolName string) error {
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

func (h *Hub) pid() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if p := h.ws.Current(); p != nil {
		return p.ID
	}
	return ""
}

func (h *Hub) activeSessionKey() string {
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

func (h *Hub) AnnounceSidebarAssistant(text string) {
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

func (h *Hub) AnnounceQueueEnqueued(job deskqueue.Job) {
	if !session.IsHiddenSession(job.SessionKey) {
		return
	}
	h.AnnounceSidebarAssistant(deskqueue.FormatSidebarEnqueue(job))
}

func (h *Hub) AnnounceQueueProgress(job deskqueue.Job, kind deskqueue.ProgressKind) {
	if !session.IsHiddenSession(job.SessionKey) {
		return
	}
	h.AnnounceSidebarAssistant(deskqueue.FormatSidebarProgress(job, kind))
}

func (h *Hub) pathExistsForAgent(rel string) bool {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" {
		return false
	}
	_, err := h.ws.ReadText(rel)
	return err == nil
}
