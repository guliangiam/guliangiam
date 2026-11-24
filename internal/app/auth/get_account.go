package auth

import (
	"context"

	domainuow "github.com/guliangiam/guliangiam/internal/domain/uow"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// GetAccount 根据 session context 查完整账号信息（org + user + 权限）。
//
// ⚠️ RLS 绕过同 Login / ResolveSessionContext：
// handler 注入的 SessionIdentity 有 OrganizationSlug + Username，
// 但 ctx 里没有 current_organization_id Postgres GUC。
// 直接在 Pool repo 上调 Organizations().GetBySlug / Users().GetByUsername
// → FORCE RLS tenant_isolation fail-closed → 404 "organization not found"。
//
// 修复：包 WithinTransaction + tenantctx.System → setTenantContext 注入
// super_admin role → tx.Organizations() / tx.Users() 事务内查询 RLS 放行。
func (s *Service) GetAccount(ctx context.Context, query AccountQuery) (*AccountContext, error) {
	if err := validateAccountQuery(query); err != nil {
		return nil, err
	}

	systemCtx := tenantctx.System(ctx)

	var result *AccountContext

	err := s.UOW.WithinTransaction(systemCtx, func(txCtx context.Context, tx domainuow.UnitOfWork) error {
		organization, err := tx.Organizations().GetBySlug(txCtx, query.OrganizationSlug)
		if err != nil {
			return err
		}
		user, err := tx.Users().GetByUsername(txCtx, organization.ID, query.Username)
		if err != nil {
			return err
		}
		acct := toAccountContext(organization, user, query.AccessToken)
		result = &acct
		return nil
	})
	if err != nil {
		return nil, wrapTransactionError(err, "get account transaction")
	}
	return result, nil
}
