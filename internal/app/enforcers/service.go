package enforcers

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type ListEnforcersQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type CreateEnforcerCommand struct {
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	ModelID        string
	AdapterID      string
	IsEnabled      *bool
	// Phase 2 fields
	Owner string
}

type PatchEnforcerCommand struct {
	ID          string
	DisplayName *string
	Description *string
	ModelID     *string
	AdapterID   *string
	IsEnabled   *bool
	Revision    int64
	// Phase 2 fields
	Owner *string
}

type EnforceCommand struct {
	ID    string
	PType string
	V0    string
	V1    string
	V2    string
	V3    string
	V4    string
	V5    string
}

type BatchEnforceCommand struct {
	ID       string
	Requests []EnforceCommand
}

func (s Service) ListEnforcers(ctx context.Context, query ListEnforcersQuery) ([]EnforcerListItem, int64, error) {
	enforcers, total, err := s.UOW.Enforcers().List(ctx, domain.ListQuery(query))
	if err != nil {
		return nil, 0, err
	}
	items := make([]EnforcerListItem, 0, len(enforcers))
	for _, e := range enforcers {
		items = append(items, toListItem(e))
	}
	return items, total, nil
}

func (s Service) GetEnforcer(ctx context.Context, organizationID, id string) (*EnforcerDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// 租户谓词在仓储层显式生效；casbin_rules 无 org 列/RLS，归属裁决必须先于任何策略操作。
	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	detail := toDetail(e)
	return &detail, nil
}

func (s Service) CreateEnforcer(ctx context.Context, command CreateEnforcerCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	isEnabled := true
	if command.IsEnabled != nil {
		isEnabled = *command.IsEnabled
	}

	enforcer := &domain.Enforcer{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		DisplayName:    command.DisplayName,
		Description:    command.Description,
		ModelID:        command.ModelID,
		AdapterID:      command.AdapterID,
		IsEnabled:      isEnabled,
		Owner:          command.Owner,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// modelId/adapterId 归属校验（BE-D-12）：跨租户引用在写入前拒绝。
		if err := validateEnforcerRefs(ctx, tx, command.OrganizationID, command.ModelID, command.AdapterID); err != nil {
			return err
		}
		if err := tx.Enforcers().Create(ctx, enforcer); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			enforcer.OrganizationID, "",
			"enforcers.created", "enforcers", enforcer.ID,
			"enforcers", "enforcers.created",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func (s Service) PatchEnforcer(ctx context.Context, organizationID string, command PatchEnforcerCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// 事务内先做 scoped 读取；跨租户在写与审计之前就返回 NotFound。
		enforcer, err := tx.Enforcers().GetByIDForOrganization(ctx, command.ID, organizationID)
		if err != nil {
			return err
		}
		if command.Revision > 0 && enforcer.Revision != command.Revision {
			return apperrors.NewConflict("enforcer revision mismatch")
		}
		if command.DisplayName != nil {
			enforcer.DisplayName = *command.DisplayName
		}
		if command.Description != nil {
			enforcer.Description = *command.Description
		}
		if command.ModelID != nil {
			enforcer.ModelID = *command.ModelID
		}
		if command.AdapterID != nil {
			enforcer.AdapterID = *command.AdapterID
		}
		if command.IsEnabled != nil {
			enforcer.IsEnabled = *command.IsEnabled
		}
		// Phase 2 field patching
		if command.Owner != nil {
			enforcer.Owner = *command.Owner
		}
		// 变更后的 modelId/adapterId 同样必须归属本租户（BE-D-12）。
		if command.ModelID != nil || command.AdapterID != nil {
			if err := validateEnforcerRefs(ctx, tx, enforcer.OrganizationID, enforcer.ModelID, enforcer.AdapterID); err != nil {
				return err
			}
		}
		enforcer.UpdatedAt = s.Clock.Now()
		enforcer.Revision++
		if err := tx.Enforcers().Update(ctx, enforcer); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			enforcer.OrganizationID, "",
			"enforcers.updated", "enforcers", enforcer.ID,
			"enforcers", "enforcers.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

func (s Service) DeleteEnforcer(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		enforcer, err := tx.Enforcers().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Enforcers().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			enforcer.OrganizationID, "",
			"enforcers.deleted", "enforcers", id,
			"enforcers", "enforcers.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// Enforce performs a policy check using the Casbin enforcer.
func (s Service) Enforce(ctx context.Context, organizationID string, command EnforceCommand) (*EnforceResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(command.ID) == "" {
		return nil, apperrors.NewInvalidArgument("enforcer id is required")
	}

	// casbin_rules 无 organization 列/RLS，必须先按可信租户取父 enforcer。
	enforcer, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, command.ID, organizationID)
	if err != nil {
		return nil, err
	}

	if !enforcer.IsEnabled {
		return nil, apperrors.NewForbidden("enforcer is disabled")
	}

	modelText, err := s.getModelText(ctx, enforcer.OrganizationID, enforcer.ModelID)
	if err != nil {
		return nil, err
	}

	rvals := buildEnforceRvals(command)
	allow, err := s.CasbinManager.Enforce(ctx, enforcer.ID, modelText, rvals...)
	if err != nil {
		return nil, apperrors.NewInternal("enforce failed: " + err.Error())
	}
	return &EnforceResult{Allow: allow}, nil
}

// BatchEnforce performs multiple policy checks using the Casbin enforcer.
func (s Service) BatchEnforce(ctx context.Context, organizationID string, command BatchEnforceCommand) (*BatchEnforceResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(command.ID) == "" {
		return nil, apperrors.NewInvalidArgument("enforcer id is required")
	}

	enforcer, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, command.ID, organizationID)
	if err != nil {
		return nil, err
	}

	if !enforcer.IsEnabled {
		return nil, apperrors.NewForbidden("enforcer is disabled")
	}

	modelText, err := s.getModelText(ctx, enforcer.OrganizationID, enforcer.ModelID)
	if err != nil {
		return nil, err
	}

	rvals := make([][]interface{}, len(command.Requests))
	for i := range command.Requests {
		rvals[i] = buildEnforceRvals(command.Requests[i])
	}

	results, err := s.CasbinManager.BatchEnforce(ctx, enforcer.ID, modelText, rvals)
	if err != nil {
		return nil, apperrors.NewInternal("batch enforce failed: " + err.Error())
	}
	return &BatchEnforceResult{Results: results}, nil
}

func (s Service) invalidatePermissionCache() {
	if s.CacheInvalidator != nil {
		s.CacheInvalidator.InvalidateAllPermissionCaches()
	}
}

// AddPolicy adds a policy rule to the enforcer.
func (s Service) AddPolicy(ctx context.Context, organizationID, enforcerID string, params []string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return err
	}
	modelText, err := s.getModelText(ctx, e.OrganizationID, e.ModelID)
	if err != nil {
		return err
	}
	if err := s.CasbinManager.AddPolicy(enforcerID, modelText, params); err != nil {
		return err
	}
	s.invalidatePermissionCache()
	return s.auditPolicyWrite(ctx, e.OrganizationID, enforcerID, "enforcers.policy.added", map[string]any{
		"policy": params,
	})
}

// RemovePolicy removes a policy rule from the enforcer.
func (s Service) RemovePolicy(ctx context.Context, organizationID, enforcerID string, params []string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return err
	}
	modelText, err := s.getModelText(ctx, e.OrganizationID, e.ModelID)
	if err != nil {
		return err
	}
	if err := s.CasbinManager.RemovePolicy(enforcerID, modelText, params); err != nil {
		return err
	}
	s.invalidatePermissionCache()
	return s.auditPolicyWrite(ctx, e.OrganizationID, enforcerID, "enforcers.policy.removed", map[string]any{
		"policy": params,
	})
}

// GetAllSubjects returns all subjects in the enforcer's policy.
func (s Service) GetAllSubjects(ctx context.Context, organizationID, enforcerID string) ([]string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return nil, err
	}
	modelText, err := s.getModelText(ctx, e.OrganizationID, e.ModelID)
	if err != nil {
		return nil, err
	}
	return s.CasbinManager.GetAllSubjects(enforcerID, modelText)
}

// GetAllObjects returns all objects in the enforcer's policy.
func (s Service) GetAllObjects(ctx context.Context, organizationID, enforcerID string) ([]string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return nil, err
	}
	modelText, err := s.getModelText(ctx, e.OrganizationID, e.ModelID)
	if err != nil {
		return nil, err
	}
	return s.CasbinManager.GetAllObjects(enforcerID, modelText)
}

// GetAllActions returns all actions in the enforcer's policy.
func (s Service) GetAllActions(ctx context.Context, organizationID, enforcerID string) ([]string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return nil, err
	}
	modelText, err := s.getModelText(ctx, e.OrganizationID, e.ModelID)
	if err != nil {
		return nil, err
	}
	return s.CasbinManager.GetAllActions(enforcerID, modelText)
}

// GetAllRoles returns all roles in the enforcer's policy.
func (s Service) GetAllRoles(ctx context.Context, organizationID, enforcerID string) ([]string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return nil, err
	}
	modelText, err := s.getModelText(ctx, e.OrganizationID, e.ModelID)
	if err != nil {
		return nil, err
	}
	return s.CasbinManager.GetAllRoles(enforcerID, modelText)
}

// ReloadPolicy forces a Casbin policy reload from the adapter for the given enforcer.
// It invalidates the cached enforcer so that policies are reloaded from the database.
func (s Service) ReloadPolicy(ctx context.Context, organizationID, enforcerID string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if strings.TrimSpace(enforcerID) == "" {
		return apperrors.NewInvalidArgument("enforcer id is required")
	}

	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return err
	}

	modelText, err := s.getModelText(ctx, e.OrganizationID, e.ModelID)
	if err != nil {
		return err
	}

	// Reload the policy from the adapter into the cached enforcer.
	if err := s.CasbinManager.ReloadPolicy(enforcerID, modelText); err != nil {
		return err
	}
	return s.auditPolicyWrite(ctx, e.OrganizationID, enforcerID, "enforcers.policy.reloaded", map[string]any{})
}

// GetPolicies returns all policy rules from the enforcer.
func (s Service) GetPolicies(ctx context.Context, organizationID, enforcerID string) ([][]string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return nil, err
	}
	modelText, err := s.getModelText(ctx, e.OrganizationID, e.ModelID)
	if err != nil {
		return nil, err
	}
	return s.CasbinManager.GetPolicies(enforcerID, modelText)
}

// GetFilteredPolicies returns policy rules matching the given field filters.
func (s Service) GetFilteredPolicies(ctx context.Context, organizationID, enforcerID string, fieldIndex int, fieldValues ...string) ([][]string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return nil, err
	}
	modelText, err := s.getModelText(ctx, e.OrganizationID, e.ModelID)
	if err != nil {
		return nil, err
	}
	return s.CasbinManager.GetFilteredPolicies(enforcerID, modelText, fieldIndex, fieldValues...)
}

// UpdatePolicy replaces an old policy rule with a new one in the enforcer.
func (s Service) UpdatePolicy(ctx context.Context, organizationID, enforcerID string, oldPolicy, newPolicy []string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	e, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return err
	}
	modelText, err := s.getModelText(ctx, e.OrganizationID, e.ModelID)
	if err != nil {
		return err
	}
	if err := s.CasbinManager.UpdatePolicy(enforcerID, modelText, oldPolicy, newPolicy); err != nil {
		return apperrors.NewInternal("failed to update policy: " + err.Error())
	}
	s.invalidatePermissionCache()
	return s.auditPolicyWrite(ctx, e.OrganizationID, enforcerID, "enforcers.policy.updated", map[string]any{
		"oldPolicy": oldPolicy,
		"newPolicy": newPolicy,
	})
}

// RefreshEngines invalidates all cached Casbin enforcer instances, forcing fresh reloads.
func (s Service) RefreshEngines(ctx context.Context) error {
	s.CasbinManager.InvalidateAll()
	return nil
}

// getModelText fetches the Casbin model text for the given model ID, scoped to
// the trusted organization: a cross-tenant modelId must not resolve (BE-D-12).
func (s Service) getModelText(ctx context.Context, organizationID, modelID string) (string, error) {
	if modelID == "" {
		return "", apperrors.NewForbidden("enforcer has no model assigned")
	}
	m, err := s.UOW.Models().GetByIDForOrganization(ctx, modelID, organizationID)
	if err != nil {
		return "", err
	}
	return m.ModelText, nil
}

// validateEnforcerRefs proves modelId/adapterId belong to the trusted tenant
// before any enforcer write. Empty refs are allowed (unassigned enforcer).
func validateEnforcerRefs(ctx context.Context, tx appuow.UnitOfWork, organizationID, modelID, adapterID string) error {
	if strings.TrimSpace(modelID) != "" {
		if _, err := tx.Models().GetByIDForOrganization(ctx, modelID, organizationID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(adapterID) != "" {
		if _, err := tx.Adapters().GetByID(ctx, adapterID, organizationID); err != nil {
			return err
		}
	}
	return nil
}

// buildEnforceRvals converts an EnforceCommand to a slice of interface{} values.
func buildEnforceRvals(cmd EnforceCommand) []interface{} {
	rvals := []interface{}{cmd.V0, cmd.V1}
	if cmd.V2 != "" {
		rvals = append(rvals, cmd.V2)
	}
	if cmd.V3 != "" {
		rvals = append(rvals, cmd.V3)
	}
	if cmd.V4 != "" {
		rvals = append(rvals, cmd.V4)
	}
	if cmd.V5 != "" {
		rvals = append(rvals, cmd.V5)
	}
	return rvals
}

// ExportPolicies exports all policies for an enforcer as a JSON-serializable structure.
func (s Service) ExportPolicies(ctx context.Context, organizationID, enforcerID string) (*PolicyExport, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	enforcer, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return nil, err
	}

	modelText, err := s.getModelText(ctx, enforcer.OrganizationID, enforcer.ModelID)
	if err != nil {
		return nil, err
	}

	policies, err := s.CasbinManager.GetPolicies(enforcerID, modelText)
	if err != nil {
		return nil, err
	}

	return &PolicyExport{
		EnforcerID:   enforcerID,
		EnforcerName: enforcer.Name,
		Policies:     policies,
		ExportedAt:   s.Clock.Now(),
	}, nil
}

// ImportPolicies imports policies into an enforcer, replacing all existing policies.
func (s Service) ImportPolicies(ctx context.Context, organizationID, enforcerID string, policies [][]string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	enforcer, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return err
	}

	if !enforcer.IsEnabled {
		return apperrors.NewForbidden("enforcer is disabled")
	}

	modelText, err := s.getModelText(ctx, enforcer.OrganizationID, enforcer.ModelID)
	if err != nil {
		return err
	}

	// 原子化替换：先校验全量，失败补偿恢复快照（BE-D-8）。
	if err := s.replacePolicies(enforcerID, modelText, policies); err != nil {
		return err
	}
	s.invalidatePermissionCache()

	return s.auditPolicyWrite(ctx, enforcer.OrganizationID, enforcerID, "enforcers.policy.imported", map[string]any{
		"count": len(policies),
	})
}

// PolicyExport represents an exportable snapshot of enforcer policies.
type PolicyExport struct {
	EnforcerID   string     `json:"enforcerId"`
	EnforcerName string     `json:"enforcerName"`
	Policies     [][]string `json:"policies"`
	ExportedAt   time.Time  `json:"exportedAt"`
}

// PolicyUploadResult contains the result of a policy Excel upload.
type PolicyUploadResult struct {
	Total     int      `json:"total"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
}
