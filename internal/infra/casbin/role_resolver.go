package casbin

import (
	"context"
	"fmt"
	"sync"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	roledomain "github.com/guliangiam/guliangiam/internal/domain/roles"
)

// RoleResolver resolves role inheritance hierarchies at runtime,

// Instead of persisting grouping (g) rules in the database,
// it resolves role hierarchies from the role table dynamically.
type RoleResolver struct {
	mu       sync.RWMutex
	cache    map[string][]string // orgID -> resolved role groupings
	roleRepo roledomain.Repository
}

// NewRoleResolver creates a new RoleResolver.
func NewRoleResolver(roleRepo roledomain.Repository) *RoleResolver {
	return &RoleResolver{
		cache:    make(map[string][]string),
		roleRepo: roleRepo,
	}
}

// ResolveRoles resolves all role inheritance groupings for an organization.
// It recursively resolves nested roles with cycle detection (max depth 10).
// Returns a list of grouping policy strings in the format "childRole, parentRole".
func (r *RoleResolver) ResolveRoles(ctx context.Context, orgID string) ([]string, error) {
	r.mu.RLock()
	if cached, ok := r.cache[orgID]; ok {
		r.mu.RUnlock()
		return cached, nil
	}
	r.mu.RUnlock()

	roles, _, err := r.roleRepo.List(ctx, roledomain.ListQuery{
		OrganizationID: orgID,
		PageSize:       1000,
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list roles for org %s", orgID)
	}

	// Build a name->role lookup map
	roleByID := make(map[string]*roledomain.Role, len(roles))
	for _, role := range roles {
		roleByID[role.ID] = role
	}

	var groupings []string
	visited := make(map[string]bool)

	for _, role := range roles {
		// Each role's RoleIDs contains the IDs of roles it inherits from
		for _, parentRoleID := range role.RoleIDs {
			parentRole, ok := roleByID[parentRoleID]
			if !ok {
				continue
			}
			groupings = append(groupings, fmt.Sprintf("%s, %s", role.Name, parentRole.Name))

			// Recursively resolve nested inheritance
			nested := r.resolveNestedRoles(roleByID, parentRole.Name, visited, 1)
			groupings = append(groupings, nested...)
		}
	}

	r.mu.Lock()
	r.cache[orgID] = groupings
	r.mu.Unlock()

	return groupings, nil
}

// resolveNestedRoles recursively resolves role inheritance with cycle detection.
// maxDepth limits recursion to prevent infinite loops.
func (r *RoleResolver) resolveNestedRoles(roleByID map[string]*roledomain.Role, roleName string, visited map[string]bool, depth int) []string {
	if depth > 10 {
		return nil
	}
	if visited[roleName] {
		return nil // cycle detected
	}
	visited[roleName] = true

	var groupings []string

	for _, role := range roleByID {
		if role.Name == roleName {
			for _, parentRoleID := range role.RoleIDs {
				parentRole, ok := roleByID[parentRoleID]
				if !ok {
					continue
				}
				groupings = append(groupings, fmt.Sprintf("%s, %s", roleName, parentRole.Name))
				nested := r.resolveNestedRoles(roleByID, parentRole.Name, visited, depth+1)
				groupings = append(groupings, nested...)
			}
		}
	}

	return groupings
}

// Invalidate clears the cache for the given organization.
func (r *RoleResolver) Invalidate(orgID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, orgID)
}

// InvalidateAll clears the entire role resolution cache.
func (r *RoleResolver) InvalidateAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(map[string][]string)
}
