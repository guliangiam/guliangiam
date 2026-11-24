package transactions

import (
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.AuditUoW
}

// 租户作用域说明：本包不做 RLS 提升。transactions 只读列表按 query 的 organizationId
// 过滤（chi.go:23）；跨组织对账查询由 ScopeAuthorizer 与 RLS 约束。
func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}
