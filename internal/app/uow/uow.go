// ============================================================================
// Unit of Work 端口接口 — 归属 domain/uow
// ============================================================================
//
// 所有 UoW 子接口（IdentityUoW / AuthUoW / UnitOfWork / TxRunner / ...）
// 统一定义在 internal/domain/uow/uow.go（Clean Architecture 依赖反转：
// 接口归属 domain，infra/postgres/mysql/uow 提供实现）。
//
// internal/app/uow 仅保留 EnsureTxRunner 辅助函数（app 层组合逻辑用）
// + 对 domain/uow 的类型别名桥接，保持现有代码向后兼容。
// ============================================================================

package uow

import (
	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
)

// 所有 UoW 接口从 domain/uow 重导出，保持 app 层消费者无需改动。
type (
	IdentityUoW      = domainuow.IdentityUoW
	AuthUoW          = domainuow.AuthUoW
	AuthnUoW         = domainuow.AuthnUoW
	AuditUoW         = domainuow.AuditUoW
	CommerceUoW      = domainuow.CommerceUoW
	InfraUoW         = domainuow.InfraUoW
	AuthzUoW         = domainuow.AuthzUoW
	RiskUoW          = domainuow.RiskUoW
	IDVUoW           = domainuow.IDVUoW
	ActionsUoW       = domainuow.ActionsUoW
	MeteringUoW      = domainuow.MeteringUoW
	WorkloadUoW      = domainuow.WorkloadUoW
	IGAUoW           = domainuow.IGAUoW
	PlatformAdminUoW = domainuow.PlatformAdminUoW
	UnitOfWork       = domainuow.UnitOfWork
	TxRunner         = domainuow.TxRunner
	// TenantProvisioningUoW 是租户开通/退役的窄能力接口（organizations 服务按需断言）。
	TenantProvisioningUoW = domainuow.TenantProvisioningUoW
	// SessionCacheUoW 是鉴权读路径的会话缓存能力（缓存命中时无需开启事务）。
	SessionCacheUoW = domainuow.SessionCacheUoW
	// UserNotificationsUoW 是个人通知中心的窄能力接口（app/usernotifications 按需断言）。
	UserNotificationsUoW = domainuow.UserNotificationsUoW
)

// EnsureTxRunner 从任意域 UoW 接口提取事务执行器。
// 如果底层实现未提供事务能力则返回 (nil, false)。
func EnsureTxRunner(uow any) (TxRunner, bool) {
	if tx, ok := uow.(TxRunner); ok {
		return tx, true
	}
	return nil, false
}
