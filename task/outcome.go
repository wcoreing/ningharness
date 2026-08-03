// 工具回执语义：失败 / 已落盘 / 已入队（未落盘）。
// 与前端 toolResultLooksFailed / CONSTITUTION 成功回执前缀对齐；勿子串匹配裸 "error"（会误伤 pauseOnError）。
package task

import "strings"

// LooksFailed 回执是否表示工具失败（红叉 / Step.OK=false）。
func LooksFailed(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(t), "error:") {
		return true
	}
	if strings.Contains(t, "❌") {
		return true
	}
	// Hub Invoke 等："error: name: …" 已由前缀覆盖；中间态 ": error:"（少见）
	if idx := strings.Index(strings.ToLower(t), ": error:"); idx >= 0 {
		return true
	}
	return false
}

// LooksDiskOK 是否为写盘成功回执（write_file / edit 等）。
func LooksDiskOK(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" || LooksFailed(t) {
		return false
	}
	return strings.Contains(t, "Successfully wrote") ||
		strings.Contains(t, "Successfully edited") ||
		strings.Contains(t, "已写入") ||
		strings.Contains(t, "已编辑")
}

// LooksQueued 是否为入队成功且未落盘（≠ 写盘成功）。
func LooksQueued(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" || LooksFailed(t) {
		return false
	}
	return strings.Contains(t, "已入队（未落盘）") || strings.Contains(t, "已入队")
}
