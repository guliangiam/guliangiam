package enforcers

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domainpermissions "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock         clock.Clock
	UUID          platformuuid.Generator
	UOW           uow.InfraUoW
	CasbinManager domainpermissions.CasbinPolicyManager
	// CacheInvalidator 在策略写成功后清除账户权限缓存（DEF-16）。
	CacheInvalidator PermissionCacheInvalidator
}

// PermissionCacheInvalidator 是 auth 服务权限缓存的窄接口（DEF-16）。
type PermissionCacheInvalidator interface {
	InvalidateAllPermissionCaches()
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.InfraUoW, casbinManager domainpermissions.CasbinPolicyManager) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork, CasbinManager: casbinManager}
}
