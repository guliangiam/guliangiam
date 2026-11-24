// Package actorctx 携带 transport 从已认证会话解析出的可信操作者 ID。
// 审计 actor 归因（BE-D-13）单一通道：会话中间件注入一次，各 app 模块
// 的 embedAuditEvent 在显式 actorID 为空时回落读取；query/body 传入的
// 任何 actor 字段都不得进入此通道。
package actorctx

import "context"

type actorContextKey struct{}

// WithActorID 注入可信操作者 ID；空值不注入。
func WithActorID(ctx context.Context, actorID string) context.Context {
	if actorID == "" {
		return ctx
	}
	return context.WithValue(ctx, actorContextKey{}, actorID)
}

// FromContext 返回可信操作者 ID；无会话时为空字符串。
func FromContext(ctx context.Context) string {
	if v, ok := ctx.Value(actorContextKey{}).(string); ok {
		return v
	}
	return ""
}
