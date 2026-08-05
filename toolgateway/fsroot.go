package toolgateway

import (
	"fmt"
	"os"
	"strings"
)

func (h *Gateway) SetFSRoot(_ string) {}
func (h *Gateway) BeginFSRoot(_ string) {}
func (h *Gateway) EndFSRoot()           {}
func (h *Gateway) FSRoot() string       { return "" }

func (h *Gateway) projectRoot() (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.projectRootLocked()
}

func (h *Gateway) projectRootLocked() (string, error) {
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

func (h *Gateway) root() (string, error) {
	return h.projectRoot()
}

func (h *Gateway) contentRoot() (string, error) {
	return h.projectRoot()
}

func (h *Gateway) Root() (string, error)        { return h.root() }
func (h *Gateway) ProjectRoot() (string, error) { return h.projectRoot() }
func (h *Gateway) ContentRoot() (string, error) { return h.contentRoot() }

func (h *Gateway) readContentText(rel string) (string, error) {
	return h.ws.ReadText(rel)
}

func (h *Gateway) mkdirContent(rel, writeID string) error {
	return h.ws.Mkdir(rel, writeID)
}

func (h *Gateway) createFileContent(rel, writeID string) error {
	return h.ws.CreateFile(rel, writeID)
}
