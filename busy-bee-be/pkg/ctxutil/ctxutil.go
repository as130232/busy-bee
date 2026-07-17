// Package ctxutil 提供跨層傳遞的 context 欄位存取（request ID 等）。
package ctxutil

import "context"

type ctxKey int

const (
	keyRequestID ctxKey = iota
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

// RequestID 取出 request ID；不存在時回空字串。
func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(keyRequestID).(string)
	return v
}
