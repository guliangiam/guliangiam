package permissions

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock         clock.Clock
	UUID          platformuuid.Generator
	UOW           uow.UnitOfWork
	CasbinManager domain.CasbinPolicyManager
	// CacheInvalidator 在权限/策略写成功后清除账户权限缓存（DEF-16）。
	CacheInvalidator PermissionCacheInvalidator
}

// PermissionCacheInvalidator 是 auth 服务权限缓存的窄接口；nil 表示不启用缓存失效。
type PermissionCacheInvalidator interface {
	InvalidateAllPermissionCaches()
}

func (s *Service) invalidatePermissionCache() {
	if s != nil && s.CacheInvalidator != nil {
		s.CacheInvalidator.InvalidateAllPermissionCaches()
	}
}

// 安全边界：PEP 中间件（APIAuthorizationMiddleware）已通过 Casbin 验证了权限。
// RLS 提升只在 service 内部生效（事务内 LOCAL set_config），事务结束自动清除。
type CreatePermissionCommand struct {
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	Users          []string
	Groups         []string
	Roles          []string
	Domains        []string
	ResourceType   string
	Resources      []string
	Actions        []string
	Effect         string
	Model          string
	Adapter        string
	Sorter         string
	Application    string
	IsEnabled      *bool
	Submitter      string
	Approver       string
	State          string
	ApproveTime    *time.Time
}

func (s *Service) NewPermission(command CreatePermissionCommand) (*domain.Permission, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	isEnabled := true
	if command.IsEnabled != nil {
		isEnabled = *command.IsEnabled
	}
	effect := normalizeEffect(command.Effect)
	state := normalizeState(command.State)

	return &domain.Permission{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		DisplayName:    command.DisplayName,
		Description:    command.Description,
		Users:          append([]string(nil), command.Users...),
		Groups:         append([]string(nil), command.Groups...),
		Roles:          append([]string(nil), command.Roles...),
		Domains:        append([]string(nil), command.Domains...),
		ResourceType:   command.ResourceType,
		Resources:      append([]string(nil), command.Resources...),
		Actions:        append([]string(nil), command.Actions...),
		Effect:         effect,
		Model:          command.Model,
		Adapter:        command.Adapter,
		Sorter:         command.Sorter,
		Application:    command.Application,
		IsEnabled:      isEnabled,
		Submitter:      command.Submitter,
		Approver:       command.Approver,
		State:          state,
		ApproveTime:    command.ApproveTime,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}, nil
}

// EnforceRequest represents a single policy evaluation request.
type EnforceRequest struct {
	Sub string // subject
	Dom string // domain
	Obj string // object
	Act string // action
}

// Enforce performs a single policy check using the specified Casbin enforcer.
// The enforcer and its model must both belong to the trusted tenant; a foreign
// or missing id is reported as not found before Casbin is touched.
func (s *Service) Enforce(ctx context.Context, organizationID, enforcerID string, request EnforceRequest) (bool, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return false, err
	}
	if strings.TrimSpace(enforcerID) == "" {
		return false, apperrors.NewInvalidArgument("enforcerId is required")
	}

	modelText, err := s.getModelTextForOrganization(ctx, organizationID, enforcerID)
	if err != nil {
		return false, err
	}

	rvals := buildEnforceRvals(request)
	allowed, err := s.CasbinManager.Enforce(ctx, enforcerID, modelText, rvals...)
	if err != nil {
		return false, apperrors.NewInternal("enforce failed: " + err.Error())
	}

	return allowed, nil
}

// BatchEnforce performs multiple policy checks using the specified Casbin enforcer.
// Every request shares the one enforcer resolved under the trusted tenant; the
// body cannot widen the tenant per item.
func (s *Service) BatchEnforce(ctx context.Context, organizationID, enforcerID string, requests []EnforceRequest) ([]bool, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(enforcerID) == "" {
		return nil, apperrors.NewInvalidArgument("enforcerId is required")
	}

	modelText, err := s.getModelTextForOrganization(ctx, organizationID, enforcerID)
	if err != nil {
		return nil, err
	}

	rvals := make([][]interface{}, len(requests))
	for i, req := range requests {
		rvals[i] = buildEnforceRvals(req)
	}

	results, err := s.CasbinManager.BatchEnforce(ctx, enforcerID, modelText, rvals)
	if err != nil {
		return nil, apperrors.NewInternal("batch enforce failed: " + err.Error())
	}

	return results, nil
}

// PolicyItem represents a single Casbin policy rule.
type PolicyItem struct {
	PType string   `json:"pType"`
	Rules []string `json:"rules"`
}

// permissionEnforcerID extracts the enforcer ID from a permission's Resources slice.
// It returns an error if the Resources slice is empty or the first element is blank.
func permissionEnforcerID(perm *domain.Permission) (string, error) {
	if len(perm.Resources) == 0 {
		return "", apperrors.NewInvalidArgument("permission has no associated enforcer: resources is empty")
	}
	enforcerID := perm.Resources[0]
	if strings.TrimSpace(enforcerID) == "" {
		return "", apperrors.NewInvalidArgument("permission has no associated enforcer")
	}
	return enforcerID, nil
}

// GetPolicies returns the Casbin policies for a permission's associated enforcer.
func (s *Service) GetPolicies(ctx context.Context, organizationID, permissionID string) ([][]string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// 先按 id + org 取得 permission 实体；casbin_rules 无 org 列/RLS。
	permission, err := s.UOW.Permissions().GetByIDForOrganization(ctx, permissionID, organizationID)
	if err != nil {
		return nil, err
	}

	enforcerID, err := permissionEnforcerID(permission)
	if err != nil {
		return nil, err
	}

	modelText, err := s.getPolicyModelText(ctx, organizationID, enforcerID)
	if err != nil {
		return nil, err
	}

	return s.CasbinManager.GetPolicies(enforcerID, modelText)
}

// UpdatePolicies replaces all policies for a permission's associated enforcer.
func (s *Service) UpdatePolicies(ctx context.Context, organizationID, permissionID string, policies []PolicyItem) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	// 先按 id + org 取得 permission 实体；casbin_rules 无 org 列/RLS。
	permission, err := s.UOW.Permissions().GetByIDForOrganization(ctx, permissionID, organizationID)
	if err != nil {
		return err
	}

	enforcerID, err := permissionEnforcerID(permission)
	if err != nil {
		return err
	}

	modelText, err := s.getPolicyModelText(ctx, organizationID, enforcerID)
	if err != nil {
		return err
	}

	if err := s.CasbinManager.RemoveFilteredPolicy(enforcerID, modelText, 0); err != nil {
		return apperrors.NewInternal("failed to clear existing policies: " + err.Error())
	}

	for _, p := range policies {
		if len(p.Rules) == 0 {
			continue
		}
		if err := s.CasbinManager.AddPolicy(enforcerID, modelText, p.Rules); err != nil {
			return apperrors.NewInternal("failed to add policy: " + err.Error())
		}
	}

	s.invalidatePermissionCache()
	return nil
}

// getPolicyModelText resolves the enforcer referenced by a permission's legacy
// resources[0] slot and validates that both the enforcer and its model belong to
// the trusted tenant before any Casbin access. The enforcer PK is a UUID, so a
// non-UUID resource value cannot denote an enforcer: it is reported as not found
// without reaching the dialect SQL or the policy store.
func (s *Service) getPolicyModelText(ctx context.Context, organizationID, enforcerID string) (string, error) {
	if _, err := uuid.Parse(enforcerID); err != nil {
		return "", apperrors.NewNotFound("enforcer not found")
	}
	return s.getModelTextForOrganization(ctx, organizationID, enforcerID)
}

// getModelTextForOrganization resolves the enforcer and its model under the
// trusted tenant. A foreign or missing row surfaces as NotFound, so callers
// cannot use enforce or permission-policy reads as a cross-tenant oracle.
func (s *Service) getModelTextForOrganization(ctx context.Context, organizationID, enforcerID string) (string, error) {
	enforcer, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return "", err
	}

	if enforcer.ModelID == "" {
		return "", apperrors.NewForbidden("enforcer has no model assigned")
	}

	m, err := s.UOW.Models().GetByIDForOrganization(ctx, enforcer.ModelID, organizationID)
	if err != nil {
		return "", err
	}

	return m.ModelText, nil
}

// buildEnforceRvals converts an EnforceRequest to a slice of interface{} values.
func buildEnforceRvals(req EnforceRequest) []interface{} {
	rvals := []interface{}{req.Sub}
	if req.Dom != "" {
		rvals = append(rvals, req.Dom)
	}
	rvals = append(rvals, req.Obj, req.Act)
	return rvals
}

// GetAllObjects returns all objects defined in the enforcer associated with the given permission.
func (s *Service) GetAllObjects(ctx context.Context, organizationID, permissionID string) ([]string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// 模型级枚举，但归属裁决必须先于 Casbin 读取。
	permission, err := s.UOW.Permissions().GetByIDForOrganization(ctx, permissionID, organizationID)
	if err != nil {
		return nil, err
	}
	enforcerID, err := permissionEnforcerID(permission)
	if err != nil {
		return nil, err
	}
	modelText, err := s.getPolicyModelText(ctx, organizationID, enforcerID)
	if err != nil {
		return nil, err
	}
	return s.CasbinManager.GetAllObjects(enforcerID, modelText)
}

// GetAllActions returns all actions defined in the enforcer associated with the given permission.
func (s *Service) GetAllActions(ctx context.Context, organizationID, permissionID string) ([]string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// 模型级枚举，但归属裁决必须先于 Casbin 读取。
	permission, err := s.UOW.Permissions().GetByIDForOrganization(ctx, permissionID, organizationID)
	if err != nil {
		return nil, err
	}
	enforcerID, err := permissionEnforcerID(permission)
	if err != nil {
		return nil, err
	}
	modelText, err := s.getPolicyModelText(ctx, organizationID, enforcerID)
	if err != nil {
		return nil, err
	}
	return s.CasbinManager.GetAllActions(enforcerID, modelText)
}

// GetAllRoles returns all roles defined in the enforcer associated with the given permission.
func (s *Service) GetAllRoles(ctx context.Context, organizationID, permissionID string) ([]string, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// 模型级枚举，但归属裁决必须先于 Casbin 读取。
	permission, err := s.UOW.Permissions().GetByIDForOrganization(ctx, permissionID, organizationID)
	if err != nil {
		return nil, err
	}
	enforcerID, err := permissionEnforcerID(permission)
	if err != nil {
		return nil, err
	}
	modelText, err := s.getPolicyModelText(ctx, organizationID, enforcerID)
	if err != nil {
		return nil, err
	}
	return s.CasbinManager.GetAllRoles(enforcerID, modelText)
}

// syncPermissionToCasbin syncs a permission to the org-scoped Casbin enforcer.
// It adds a policy rule: p, perm.ID, domain, resource, action for each
// combination of domain, resource, and action defined in the permission.
func syncPermissionToCasbin(mgr domain.CasbinPolicyManager, orgID string, perm *domain.Permission) error {
	if mgr == nil || perm == nil || !perm.IsEnabled {
		return nil
	}

	for _, res := range perm.Resources {
		for _, act := range perm.Actions {
			for _, dom := range perm.Domains {
				if err := mgr.AddOrgPolicy(orgID, []string{perm.ID, dom, res, act}); err != nil {
					return err
				}
			}
		}
	}

	if len(perm.Domains) == 0 {
		for _, res := range perm.Resources {
			for _, act := range perm.Actions {
				if err := mgr.AddOrgPolicy(orgID, []string{perm.ID, orgID, res, act}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// removePermissionFromCasbin removes all Casbin policies associated with
// the given permission ID from the org-scoped enforcer.
func removePermissionFromCasbin(mgr domain.CasbinPolicyManager, orgID, permID string) error {
	if mgr == nil {
		return nil
	}

	return mgr.RemoveOrgFilteredPolicy(orgID, 0, permID)
}
