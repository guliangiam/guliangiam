package keys

import (
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.AuthUoW
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.AuthUoW) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

// 租户作用域说明：本包不做 RLS 提升。keys 列表按 query 的 organizationId 过滤；
// 跨组织的 /api/v1/global/keys 以空 organizationId 查询（chi.go:84），可见范围由 RLS 决定。
