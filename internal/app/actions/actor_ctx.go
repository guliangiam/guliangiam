package actions

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/platform/actorctx"
)

// WithActorID 注入可信操作者 ID（BE-D-13，共享 platform/actorctx 通道，
// 由会话中间件统一注入；此入口保留给显式调用方）。
func WithActorID(ctx context.Context, actorID string) context.Context {
	return actorctx.WithActorID(ctx, actorID)
}

func actorIDFromContext(ctx context.Context) string {
	return actorctx.FromContext(ctx)
}
