package workload

import (
	"context"
	"time"
)

// Repository 是 workload 域的端口接口。
// MySQL 实现：internal/infra/mysql/workload.Repository
// PostgreSQL 实现：internal/infra/postgres/workload.Repository
type Repository interface {
	// ── Identity Pools ──────────────────────────────────────────────
	ListPools(ctx context.Context, q ListPoolsQuery) ([]*Pool, int64, error)
	GetPool(ctx context.Context, id, orgID string) (*Pool, error)
	CreatePool(ctx context.Context, p *Pool) error

	// ── Federation Trusts ──────────────────────────────────────────
	// ListTrusts 的组织/池过滤均可选，供认证 resolver 使用；管理端必须在应用层限定组织。
	ListTrusts(ctx context.Context, q ListTrustsQuery) ([]*FederationTrust, int64, error)
	GetTrust(ctx context.Context, id, orgID string) (*FederationTrust, error)
	FindActiveTrustsByIssuer(ctx context.Context, issuer string) ([]*FederationTrust, error)
	CreateTrust(ctx context.Context, t *FederationTrust) error
	// SetTrustStatus 软启用/禁用 FederationTrust，不允许复活 deleted。
	SetTrustStatus(ctx context.Context, id, orgID string, newStatus FederationTrustStatus) error
	// DeleteTrust 软删除 FederationTrust（status → "deleted"）。
	DeleteTrust(ctx context.Context, id, orgID string) error

	// ── Service Accounts ───────────────────────────────────────────
	ListAccounts(ctx context.Context, q ListAccountsQuery) ([]*ServiceAccount, int64, error)
	GetAccount(ctx context.Context, id, orgID string) (*ServiceAccount, error)
	GetAccountByClientID(ctx context.Context, clientID string) (*ServiceAccount, error)
	GetAccountByName(ctx context.Context, orgID, name string) (*ServiceAccount, error)
	CreateAccount(ctx context.Context, a *ServiceAccount) error
	SetAccountStatus(ctx context.Context, id, orgID string, newStatus ServiceAccountStatus, expectedRevision int64) error
	RotateAccountSecret(ctx context.Context, id, orgID, secretHash string, rotatedAt time.Time, expectedRevision int64) error
	DeleteAccount(ctx context.Context, id, orgID string, expectedRevision int64) error
}
