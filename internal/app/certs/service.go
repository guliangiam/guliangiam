package certs

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domaincerts "github.com/guliangiam/guliangiam/internal/domain/certs"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock       clock.Clock
	UUID        platformuuid.Generator
	UOW         uow.AuthUoW
	ACMERenewer domaincerts.CertificateRenewer
}

// 租户作用域说明：本包不做 RLS 提升。certs 列表的 organizationId 来自 query；
// 跨组织的 /api/v1/global/certs 以空 organizationId 查询（chi.go:117），可见范围由 RLS 决定。
