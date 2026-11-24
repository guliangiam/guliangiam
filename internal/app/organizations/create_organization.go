package organizations

import (
	"context"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	enforcersdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	tenantidentities "github.com/guliangiam/guliangiam/internal/domain/tenant_identities"
	tenantinstances "github.com/guliangiam/guliangiam/internal/domain/tenant_instances"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func (s *Service) CreateOrganization(ctx context.Context, command CreateOrganizationCommand) (*orgdomain.Organization, error) {
	if err := validateCreateOrganizationCommand(command); err != nil {
		return nil, err
	}

	organization, err := s.NewOrganization(command)
	if err != nil {
		return nil, err
	}
	if err := organization.Validate(); err != nil {
		return nil, err
	}

	// 新组织 id 不满足 id = current_organization_id()：能否通过 WITH CHECK 取决于调用方作用域（platform_admin 会话为 super_admin）与 RLS，service 不提升作用域。
	// 事务边界在此显式声明（等价于 runInTx，但让 check-tenant-provisioning
	// 门禁与审查者直接看到 WithinTransaction，而不必穿透 helper）。
	runner, ok := appuow.EnsureTxRunner(s.UOW)
	if !ok {
		return nil, apperrors.NewInternal("unit of work does not support transactions")
	}
	if err := runner.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Check for active organization with the same slug.
		existing, err := tx.Organizations().GetBySlug(ctx, organization.Slug)
		if err == nil && existing != nil {
			return apperrors.NewAlreadyExists("organization slug already exists")
		}
		if appErr, ok := err.(*apperrors.Error); !ok || appErr.Code != apperrors.CodeNotFound {
			return err
		}

		// Check for soft-deleted organization with the same slug.
		// The DB unique constraint on slug does not include deleted_at, so a
		// soft-deleted record would cause a constraint violation on INSERT.
		hasDeleted, err := tx.Organizations().HasDeletedWithSlug(ctx, organization.Slug)
		if err != nil {
			return err
		}
		if hasDeleted {
			return apperrors.NewAlreadyExists("organization slug already exists (soft-deleted); permanently delete the existing organization or use a different slug")
		}
		if err := validateOrganizationParentChain(ctx, tx.Organizations(), organization.ID, command.ParentID); err != nil {
			return err
		}

		// 先 Insert 主实体 — records.organization_id FK 必须引用已存在的 organizations.id
		if err := tx.Organizations().Create(ctx, organization); err != nil {
			return err
		}
		// 主表 Create 只写 4 张配置子表，settings 行按需 upsert：只有调用方显式
		// 传入 settings 字段（默认应用或语言）时才落库，避免为默认配置多写一行。
		if organization.Settings.DefaultApplicationID != "" || len(organization.Settings.Languages) > 0 {
			if err := tx.Organizations().UpdateSettings(ctx, &organization.Settings); err != nil {
				return err
			}
		}

		// 创建组织即创建租户：身份 + 主拓扑与组织行同一事务写入。
		// 任一步失败则整个事务回滚，不会留下「只有组织、没有租户」的孤儿。
		tenantTx, ok := tx.(appuow.TenantProvisioningUoW)
		if !ok {
			return apperrors.NewUnimplemented("tenant provisioning is not implemented by this unit of work")
		}
		if err := s.provisionTenant(ctx, tenantTx, organization); err != nil {
			return err
		}

		// 再写 audit + outbox（此时 org 行已在事务内，FK 能过）
		if err := s.embedAuditEvent(
			tx,
			ctx,
			organization.ID,
			command.ActorID,
			"organization.created",
			"Organization",
			organization.ID,
			"Organization",
			"control.tenant_registered",
			map[string]any{},
		); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return organization, nil
}

// provisionTenant writes the 1:1 tenant identity and the primary isolation
// topology row for a freshly inserted organization. The default region comes
// from deployment_regions (is_primary, stable order); no environment variable
// decides the placement. Unique constraints (tenant_identities PK,
// uq_tenant_instances_primary) are the concurrency backstop.
func (s *Service) provisionTenant(ctx context.Context, tx appuow.TenantProvisioningUoW, organization *orgdomain.Organization) error {
	now := organization.CreatedAt
	identity := &tenantidentities.TenantIdentity{
		OrganizationID: organization.ID,
		Status:         tenantidentities.StatusActive,
		IsolationTier:  tenantidentities.IsolationTierRow,
		IsBuiltIn:      organization.Slug == "built-in",
		ProvisionedAt:  &now,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := identity.Validate(); err != nil {
		return err
	}
	if err := tx.Tenants().CreateIdentity(ctx, identity); err != nil {
		return err
	}

	region, err := tx.Tenants().PrimaryRegion(ctx)
	if err != nil {
		return err
	}
	topologyID, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	topology := &tenantinstances.TenantInstance{
		ID:                   topologyID,
		OrganizationID:       organization.ID,
		RegionID:             region.ID,
		RegionCode:           region.RegionCode,
		IsPrimary:            true,
		Status:               tenantinstances.StatusActive,
		PlacementKind:        tenantinstances.PlacementSharedCluster,
		StorageBoundary:      tenantinstances.BoundaryShared,
		BackupBoundary:       tenantinstances.BoundaryShared,
		DataResidencyCountry: region.CountryCode,
		ProvisionedAt:        &now,
		CreatedAt:            now,
		UpdatedAt:            now,
		Revision:             1,
	}
	if err := topology.Validate(); err != nil {
		return err
	}
	if err := tx.Tenants().CreatePrimaryTopology(ctx, topology); err != nil {
		return err
	}

	// 默认 enforcer：授权中间件按 organization_id 解析 enforcer UUID，
	// 缺失时 ResolveEnforcerUUID 失败 → 该组织所有用户 500（DEF-34）。
	// model_id/adapter_id 允许为空：管理器的模型文本由 principalModel(type) 提供。
	enforcerID, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	enforcer := &enforcersdomain.Enforcer{
		ID:             enforcerID,
		OrganizationID: organization.ID,
		Name:           "default",
		DisplayName:    "Default Enforcer",
		IsEnabled:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	return tx.Enforcers().Create(ctx, enforcer)
}
