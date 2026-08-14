package toolgateway

import "strings"

// ShellUI 产品壳长寿 UI 状态（非 Turn；与投影分离）。
type ShellUI struct {
	FocusRel      string
	ContentTabs   string
	TreeSelection string
}

func (h *Gateway) SetFocus(relPath string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ui.FocusRel = strings.TrimSpace(relPath)
}

func (h *Gateway) Focus() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ui.FocusRel
}

func (h *Gateway) SetContentTabsBrief(brief string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ui.ContentTabs = strings.TrimSpace(brief)
}

func (h *Gateway) ContentTabsBrief() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ui.ContentTabs
}

func (h *Gateway) SetTreeSelectionBrief(brief string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ui.TreeSelection = strings.TrimSpace(brief)
}

func (h *Gateway) TreeSelectionBrief() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ui.TreeSelection
}

// UI 返回壳 UI 快照（拷贝）。
func (h *Gateway) UI() ShellUI {
	if h == nil {
		return ShellUI{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ui
}
