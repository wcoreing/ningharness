// Package guest Guest 契约：跑模型的一方；工具仍须经 ToolHost。
package guest

import "context"

// Guest 一句话/一轮对话入口（可选；产品可自备 Guest）。
type Guest interface {
	Chat(ctx context.Context, message string) (reply string, err error)
}
