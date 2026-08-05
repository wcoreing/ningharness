package toolgateway

import (
	"strings"

	"ningharness/trace"
)

func (h *Gateway) WithTaskTrace(root, taskID, sessionKey, jobID string, fn func() error) (err error) {
	if h == nil {
		return fn()
	}
	root = strings.TrimSpace(root)
	taskID = strings.TrimSpace(taskID)
	if root == "" || taskID == "" {
		return fn()
	}
	w, berr := trace.Begin(root, taskID, sessionKey, jobID)
	if berr != nil {
		return fn()
	}
	h.SetTaskTrace(w)
	h.ProjectTurn(taskID, sessionKey, "", jobID)
	defer func() {
		status := "ok"
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
			status = "error"
			if strings.Contains(errMsg, "context canceled") || strings.Contains(errMsg, "context cancelled") {
				status = "cancelled"
			}
		}
		h.FinishTurn(status, errMsg)
	}()
	err = fn()
	return err
}

func (h *Gateway) ArmTaskTrace(root, taskID, sessionKey, jobID string) {
	if h == nil {
		return
	}
	root = strings.TrimSpace(root)
	taskID = strings.TrimSpace(taskID)
	if root == "" || taskID == "" {
		return
	}
	w, err := trace.Begin(root, taskID, sessionKey, jobID)
	if err != nil {
		return
	}
	h.SetTaskTrace(w)
}

func (h *Gateway) DisarmTaskTrace(status, errMsg string) {
	if h == nil {
		return
	}
	w := h.TaskTrace()
	if w != nil {
		_ = w.End(status, errMsg)
	}
	h.SetTaskTrace(nil)
}
