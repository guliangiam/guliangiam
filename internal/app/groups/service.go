package groups

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.IdentityUoW
}

// 租户作用域说明：本包不做 RLS 提升。groups 的 organizationId 由 transport 层
// 用 TrustedOrganizationID 收敛（query 读、body 写、bulk-import 每行）；仓储按
// organization_id 过滤，跨组织写入需上游 ScopeAuthorizer 授权。
