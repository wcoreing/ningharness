package toolgateway

import "strings"

// ProjectTurn 将本轮身份投影到网关上，供工具回调读取。
// 应由 lifecycle begin_task 调用；真相仍在 lifecycle.RunState。
func (h *Gateway) ProjectTurn(taskID, sessionKey, parentTaskID, jobID string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.turnTaskID = strings.TrimSpace(taskID)
	h.turnSessionKey = strings.TrimSpace(sessionKey)
	h.turnParentTask = strings.TrimSpace(parentTaskID)
	h.turnJobID = strings.TrimSpace(jobID)
}

// ClearTurnProjection 清空当前轮投影（不含 Trace Writer；见 FinishTurn）。
func (h *Gateway) ClearTurnProjection() {
	h.ProjectTurn("", "", "", "")
}

// FinishTurn 一轮收尾唯一入口：收 Trace + 清投影（幂等）。
// 成功/失败都应由 RunState.OnExit 调用，避免 EndTask 与 OnExit 双写。
func (h *Gateway) FinishTurn(status, errMsg string) {
	if h == nil {
		return
	}
	h.DisarmTaskTrace(status, errMsg)
	h.ClearTurnProjection()
}
