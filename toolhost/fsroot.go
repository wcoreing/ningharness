package toolhost

import (
	"fmt"
	"os"
	"strings"
)

func (h *ToolHost) SetFSRoot(_ string) {}
func (h *ToolHost) BeginFSRoot(_ string) {}
func (h *ToolHost) EndFSRoot()           {}
func (h *ToolHost) FSRoot() string       { return "" }

func (h *ToolHost) projectRoot() (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.projectRootLocked()
}

func (h *ToolHost) projectRootLocked() (string, error) {
	p := h.ws.Current()
	if p == nil {
		if env := strings.TrimSpace(os.Getenv("AGENTDESK_PROJECT")); env != "" {
			if _, err := h.ws.Open(env); err != nil {
				return "", err
			}
			return h.ws.Current().Root, nil
		}
		return "", fmt.Errorf("no project: call use_project first or set AGENTDESK_PROJECT")
	}
	return p.Root, nil
}

func (h *ToolHost) root() (string, error) {
	return h.projectRoot()
}

func (h *ToolHost) contentRoot() (string, error) {
	return h.projectRoot()
}

func (h *ToolHost) Root() (string, error)        { return h.root() }
func (h *ToolHost) ProjectRoot() (string, error) { return h.projectRoot() }
func (h *ToolHost) ContentRoot() (string, error) { return h.contentRoot() }

func (h *ToolHost) readContentText(rel string) (string, error) {
	return h.ws.ReadText(rel)
}

func (h *ToolHost) mkdirContent(rel, writeID string) error {
	return h.ws.Mkdir(rel, writeID)
}

func (h *ToolHost) createFileContent(rel, writeID string) error {
	return h.ws.CreateFile(rel, writeID)
}
