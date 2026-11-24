package webhooks

import (
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.UnitOfWork
}

func NewService(clock clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork) Service {
	return Service{Clock: clock, UUID: uuidGenerator, UOW: unitOfWork}
}

// 租户作用域说明：本包不做 RLS 提升。webhooks 的 organizationId 由 transport 解析
// 为可信租户（query/body 里的值只对平台作用域生效）；by-ID 的读/改/删在 app 层按
// organization_id 归属裁决，跨租户统一返回 not found。PG 由 RLS + app 层归属校验
// 双兜底，MySQL 无 RLS 时仅由 app 层校验。
