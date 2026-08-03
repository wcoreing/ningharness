package hub

import (
	"fmt"
	"os"
	"strings"
)

func (h *Hub) SetFSRoot(_ string) {}
func (h *Hub) BeginFSRoot(_ string) {}
func (h *Hub) EndFSRoot()           {}
func (h *Hub) FSRoot() string       { return "" }

func (h *Hub) projectRoot() (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.projectRootLocked()
}

func (h *Hub) projectRootLocked() (string, error) {
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

func (h *Hub) root() (string, error) {
	return h.projectRoot()
}

func (h *Hub) contentRoot() (string, error) {
	return h.projectRoot()
}

func (h *Hub) Root() (string, error)        { return h.root() }
func (h *Hub) ProjectRoot() (string, error) { return h.projectRoot() }
func (h *Hub) ContentRoot() (string, error) { return h.contentRoot() }

func (h *Hub) readContentText(rel string) (string, error) {
	return h.ws.ReadText(rel)
}

func (h *Hub) mkdirContent(rel, writeID string) error {
	return h.ws.Mkdir(rel, writeID)
}

func (h *Hub) createFileContent(rel, writeID string) error {
	return h.ws.CreateFile(rel, writeID)
}
