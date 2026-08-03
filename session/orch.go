package session

import (
	"strings"
)

const MainSessionID = "main"

// IsHiddenSession 系统/写手会话：不进侧栏历史列表。
func IsHiddenSession(sessionID string) bool {
	id := strings.TrimSpace(sessionID)
	return strings.HasPrefix(id, "once:") || strings.HasPrefix(id, "skill-reflect:")
}

// OrchKey 编排键：orch:{projectId}:{sessionId}（与稿舍约定对齐）。
func OrchKey(projectID, sessionID string) string {
	projectID = strings.TrimSpace(projectID)
	sessionID = NormalizeSessionID(sessionID)
	if projectID == "" {
		return "orch:unknown:" + sessionID
	}
	return "orch:" + projectID + ":" + sessionID
}

// NormalizeSessionID 空 / default → main。
func NormalizeSessionID(raw string) string {
	key := strings.TrimSpace(raw)
	if key == "" || key == "default" {
		return MainSessionID
	}
	return key
}

// ParseOrchKey 从 orch:pid:sess 拆出 sessionId。
func ParseOrchKey(projectID, orch string) string {
	orch = strings.TrimSpace(orch)
	prefix := "orch:" + strings.TrimSpace(projectID) + ":"
	if strings.HasPrefix(orch, prefix) {
		return NormalizeSessionID(strings.TrimPrefix(orch, prefix))
	}
	if strings.HasPrefix(orch, "orch:") {
		rest := strings.TrimPrefix(orch, "orch:")
		if i := strings.Index(rest, ":"); i >= 0 {
			return NormalizeSessionID(rest[i+1:])
		}
	}
	return NormalizeSessionID(orch)
}
