package guest

import "context"

type deltaCtxKey struct{}

// WithDeltaHandler 将流式文本回调挂到 ctx（产品 Guest / Eino 均可读取）。
func WithDeltaHandler(ctx context.Context, fn func(delta string)) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, deltaCtxKey{}, fn)
}

// DeltaHandler 读取流式回调；无则 nil。
func DeltaHandler(ctx context.Context) func(delta string) {
	if ctx == nil {
		return nil
	}
	fn, _ := ctx.Value(deltaCtxKey{}).(func(string))
	return fn
}
