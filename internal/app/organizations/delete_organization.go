package organizations

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	tenantidentities "github.com/guliangiam/guliangiam/internal/domain/tenant_identities"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DeleteOrganizationCommand contains the parameters for deleting an organization.
type DeleteOrganizationCommand struct {
	ID string
}

// DeleteOrganization soft-deletes an organization by setting its deleted_at
// timestamp, and retires the tenant identity plus isolation topology in the
// same transaction. No physical delete is performed anywhere on this path.
func (s *Service) DeleteOrganization(ctx context.Context, command DeleteOrganizationCommand) error {
	if command.ID == "" {
		return apperrors.NewInvalidArgument("organization ID is required")
	}

	// command.ID 来自 URL path；软删除按 id 执行，能否命中跨 org 行由调用方作用域与 RLS USING 决定。
	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Verify organization exists
		org, err := tx.Organizations().GetByID(ctx, command.ID)
		if err != nil {
			return err
		}
		if org == nil {
			return apperrors.NewNotFound("organization not found")
		}

		// Prevent deletion of built-in organization
		if org.Slug == "built-in" {
			return apperrors.NewForbidden("cannot delete built-in organization")
		}

		now := s.Clock.Now()
		tenantTx, ok := tx.(appuow.TenantProvisioningUoW)
		if !ok {
			return apperrors.NewUnimplemented("tenant provisioning is not implemented by this unit of work")
		}
		// 租户身份退役：active/failed → decommissioned。历史组织可能还没有身份行，
		// NotFound 不阻断删除，但会补一次幂等退拓扑。
		identity, err := tenantTx.Tenants().GetIdentity(ctx, org.ID)
		switch {
		case err == nil:
			if identity.IsBuiltIn {
				return apperrors.NewForbidden("cannot decommission built-in tenant identity")
			}
			if err := identity.TransitionTo(tenantidentities.StatusDecommissioned, now); err != nil {
				return err
			}
		case isNotFound(err):
			// legacy organization without identity row: keep going
		default:
			return err
		}
		if err := tenantTx.Tenants().DecommissionIdentity(ctx, org.ID, now); err != nil {
			return err
		}
		if err := tenantTx.Tenants().DecommissionTopologies(ctx, org.ID, now); err != nil {
			return err
		}

		if err := tx.Organizations().Delete(ctx, command.ID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			org.ID,
			"",
			"organization.deleted",
			"Organization",
			org.ID,
			"Organization",
			"control.tenant_decommissioned",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func isNotFound(err error) bool {
	appErr, ok := err.(*apperrors.Error)
	return ok && appErr.Code == apperrors.CodeNotFound
}
