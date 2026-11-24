package auth

import (
	"context"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/actorctx"
)

// BE-D-13：会话注入必须同时携带可信审计 actor，供所有 app 模块回落读取。
func TestWithSessionContext_InjectsTrustedActor(t *testing.T) {
	ctx := WithSessionContext(context.Background(), SessionContext{UserID: "user-42"})
	if got := actorctx.FromContext(ctx); got != "user-42" {
		t.Fatalf("actorctx.FromContext = %q, want user-42", got)
	}
	session, ok := SessionContextFromContext(ctx)
	if !ok || session.UserID != "user-42" {
		t.Fatalf("session round-trip failed: ok=%v session=%+v", ok, session)
	}
}

func TestWithSessionContext_EmptyUserIDNoActor(t *testing.T) {
	ctx := WithSessionContext(context.Background(), SessionContext{})
	if got := actorctx.FromContext(ctx); got != "" {
		t.Fatalf("actorctx.FromContext = %q, want empty", got)
	}
}
