package platform_admins

import (
	"context"
	"errors"
	"testing"
	"time"

	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	platformerrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 越界入口回归：应用层 CreateScope 不允许创建未审批（approved_by=nil）的 scope。
// 该请求必须在任何事务/时钟依赖之前被拒绝，因此零值 Service 即可验证。
func TestCreateScope_RejectsUnapproved(t *testing.T) {
	svc := Service{}
	_, err := svc.CreateScope(context.Background(), CreateScopeCommand{
		PlatformAdminID: "01a0ea76-0c00-7000-8000-000000000019",
		ScopeType:       platformadmins.ScopeTypeAll,
		ApprovedBy:      nil,
		ExpiresAt:       time.Now().Add(24 * time.Hour),
	})
	if err == nil {
		t.Fatal("CreateScope(approved_by=nil) must be rejected")
	}
	var pe *platformerrors.Error
	if !errors.As(err, &pe) || pe.Code != platformerrors.CodeInvalidArgument {
		t.Fatalf("want CodeInvalidArgument, got %v", err)
	}
}
