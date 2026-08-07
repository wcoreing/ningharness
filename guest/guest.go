// Package guest 是 Guest 插槽：怎么想；工具须经 Gateway（本包不定义 Lifecycle）。
package guest

import (
	"context"
	"fmt"
	"strings"

	"ningharness/history"
)

// Input 一轮交给 Guest 的进模输入（不含 RunState）。
type Input struct {
	// Message 用户原话（或 Job wire prompt）。
	Message string
	// Feedforward 本轮前馈；进模时与 Message 合并（见 Wire）。
	Feedforward string
}

// Guest 可换模型循环。
type Guest interface {
	Run(ctx context.Context, in Input) (reply string, err error)
}

// Wire 前馈 + 用户原话 → 单条进模 user content。
func Wire(in Input) string {
	return history.ApplyFeedforward(in.Feedforward, strings.TrimSpace(in.Message))
}

// Chat 兼容糖：无前馈的一句话 Run。
func Chat(ctx context.Context, g Guest, message string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("guest: nil Guest")
	}
	return g.Run(ctx, Input{Message: message})
}
