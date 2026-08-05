// Package guest 模型层契约：怎么想；工具仍须经 Gateway（不拥有生命周期定义）。
package guest

import "context"

// Guest 一句话/一轮对话入口（可选；产品可自备 Guest）。
type Guest interface {
	Chat(ctx context.Context, message string) (reply string, err error)
}
