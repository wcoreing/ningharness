package history

import (
	"fmt"
	"strings"
)

// WindowInfo 进模窗口元数据（与 BuildForModel 同源，UI 只读展示）。
type WindowInfo struct {
	RecentKeep  int
	MaxMsgRunes int
	InWindowSeq map[int]bool // seq → 是否在 ContextMessages 内
}

// ContextWindow 计算进模窗口：复用 BuildForModel，禁止 duplicate 截断逻辑。
func ContextWindow(msgs []Msg, budget Budget) WindowInfo {
	budget = budget.normalize()
	in := BuildForModel(msgs, budget, true)
	set := make(map[int]bool, len(in))
	for _, m := range in {
		if m.Seq > 0 {
			set[m.Seq] = true
		}
	}
	return WindowInfo{
		RecentKeep:  budget.RecentKeep,
		MaxMsgRunes: budget.MaxMsgRunes,
		InWindowSeq: set,
	}
}

// FeedforwardBySeq 按 seq 读取 user 行前馈（按需 API，避免 Snapshot 过大）。
func FeedforwardBySeq(root, sessionKey string, seq int) (string, error) {
	if seq <= 0 {
		return "", fmt.Errorf("history: invalid seq")
	}
	msgs, err := Load(root, sessionKey)
	if err != nil {
		return "", err
	}
	for _, m := range msgs {
		if m.Seq == seq && strings.TrimSpace(m.Role) == "user" {
			return strings.TrimSpace(m.Feedforward), nil
		}
	}
	return "", fmt.Errorf("history: no user feedforward for seq %d", seq)
}
