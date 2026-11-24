package resources

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domainresources "github.com/guliangiam/guliangiam/internal/domain/resources"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock           clock.Clock
	UUID            platformuuid.Generator
	UOW             uow.UnitOfWork
	Storage         domainresources.StorageProvider
	StorageResolver domainresources.StorageProviderResolver
}

// 租户作用域说明：transport 解析可信租户后显式传入，仓储层以
// organization_id 谓词裁决归属；上传对象键由应用层生成，不接受客户端路径。
