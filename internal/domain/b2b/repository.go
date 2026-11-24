package b2b

import (
	"context"
	"time"
)

// ListQuery 查询参数。
type ListQuery struct {
	Page, PageSize int
	OrganizationID string
	Status         string // 空 = 全部；否则过滤 closed set
}

// Repository 定义 B2B 域名持久化端口接口。
//
// 所有方法按 Clean Architecture 定义在 domain 层 —— infra/postgres/b2b/
// 实现；app/b2b 通过接口依赖，不感知具体存储。
//
// 事务归属：属于 UoW 的 B2BDomains() 子仓库（domain/uow/uow.go
// B2BUoW interface）。Caller 必须在 EnsureWithinTx 内调用以保证一致性
// （同一 transaction 内写入 organization_domains + 触发 outbox 事件）。
type Repository interface {
	// Create 认领新域名。DB 层负责 (organization_id, domain) 唯一约束
	// —— 重名认领应被 SQL 唯一索引拒绝，不是应用层判断。
	Create(ctx context.Context, d *OrganizationDomain) error

	GetByID(ctx context.Context, id string) (*OrganizationDomain, error)

	// GetByIDForOrganization 按 id + organizationID 读取；跨租户与不存在同样
	// 返回 NotFound，管理面 by-ID 路由必须使用本方法而不是 GetByID。
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*OrganizationDomain, error)

	// GetByDomain 按 (organization_id, domain) 查 —— 创建前查重用。
	GetByDomain(ctx context.Context, organizationID, domain string) (*OrganizationDomain, error)

	// List 按组织列出。支持 status 过滤（pending / verified / failed）。
	List(ctx context.Context, q ListQuery) ([]*OrganizationDomain, int64, error)

	// Update 更新聚合（含状态迁移与续验预算字段）。调用方必须先调用
	// MarkVerified / MarkFailed / RecordVerificationFailure / RecordVerificationSuccess
	// —— Repository 不做业务校验，只持久化。仅 pending 行可更新；已终态化时
	// 返回 Conflict，调用方据此判定「其他实例已终态化」，不计业务失败。
	Update(ctx context.Context, d *OrganizationDomain) error

	// Delete 按 id + organizationID 删除域名认领；0 行返回 NotFound。
	Delete(ctx context.Context, id, organizationID string) error

	// FindVerifiedByDomain 邮箱域 SSO 发现入口。
	//
	// 输入：corp.example.com（从用户邮箱 alice@corp.example.com 解析）
	// 输出：已验证的 OrganizationDomain（如果有）；返回 nil 表示未认领。
	// 匹配逻辑：domain 列精确匹配 + status=verified 过滤。
	FindVerifiedByDomain(ctx context.Context, domain string) (*OrganizationDomain, error)

	// ClaimPending 原子认领至多 limit 条到期（status=pending 且 next_retry_at <= now）
	// 的域名，并把 next_retry_at 推进到 now+lease 作为租约。实现必须使用
	// FOR UPDATE SKIP LOCKED（或等价语义），保证多实例不会重复查询同一行。
	// 返回行携带最新 attempts/last_error/next_retry_at/last_checked_at。
	ClaimPending(ctx context.Context, limit int, lease time.Duration) ([]*OrganizationDomain, error)
}
