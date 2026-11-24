package permissions

import (
	"context"
	"sort"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// EffectivePermission is one resource/action grant a user receives through a
// bound role. Role names the binding the grant came from so the console can
// explain why the user holds it.
type EffectivePermission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Role     string `json:"role"`
}

// ListUserEffectivePermissions expands every role bound to the user through the
// tenant's Casbin policies (role inheritance included) and returns the
// deduplicated grants. It is a read-only, tenant-scoped query: a user id from
// another organization is reported exactly like a missing one.
func (s *Service) ListUserEffectivePermissions(ctx context.Context, organizationID, userID string) ([]EffectivePermission, error) {
	orgID := strings.TrimSpace(organizationID)
	if orgID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(userID) == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	if s.CasbinManager == nil {
		return nil, apperrors.NewUnavailable("permission policy engine is not configured")
	}

	user, err := s.UOW.Users().GetByIDForOrganization(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}

	grants := make([]EffectivePermission, 0)
	seen := make(map[string]struct{})
	for _, binding := range user.RoleBindings {
		role, err := s.UOW.Roles().GetByIDForOrganization(ctx, binding.RoleID, orgID)
		if err != nil {
			// A binding can outlive the role row for a moment during deletion;
			// skip it rather than failing the whole read.
			continue
		}
		policies, err := s.CasbinManager.GetOrgSubjectPolicies(orgID, role.Name)
		if err != nil || len(policies) == 0 {
			// casbin_rules usually stores the raw role name, but some deployments
			// normalize the subject ("<org>-<name>", lowercase). Mirror the login
			// resolver so both layouts expand to the same grants.
			actor := normalizedSubject(role.Name, orgID)
			if actor != role.Name {
				policies, err = s.CasbinManager.GetOrgSubjectPolicies(orgID, actor)
			}
		}
		if err != nil || len(policies) == 0 {
			continue
		}
		for _, policy := range policies {
			if len(policy) < 4 {
				continue
			}
			resource, action := policy[2], policy[3]
			if resource == "" || action == "" {
				continue
			}
			key := resource + "\x00" + action + "\x00" + role.Name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			grants = append(grants, EffectivePermission{Resource: resource, Action: action, Role: role.Name})
		}
	}

	sort.Slice(grants, func(i, j int) bool {
		if grants[i].Resource != grants[j].Resource {
			return grants[i].Resource < grants[j].Resource
		}
		if grants[i].Action != grants[j].Action {
			return grants[i].Action < grants[j].Action
		}
		return grants[i].Role < grants[j].Role
	})
	return grants, nil
}

// normalizedSubject mirrors the login-side Casbin subject normalization used
// when a deployment stores "<org>-<role>" lowercased instead of the raw name.
func normalizedSubject(slug, orgID string) string {
	actor := strings.TrimSpace(slug)
	actor = strings.TrimPrefix(actor, orgID+"-")
	actor = strings.TrimPrefix(actor, "built-in-")
	return strings.ToLower(actor)
}
