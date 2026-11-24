package groups

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
)

// queryGetGroupByNameForOrganization is the exact-name lookup used by LDAP
// group upserts. Keyword list scans are substring matches and force the caller
// to walk every matching page; an equality predicate resolves the group in a
// single indexed read.
const queryGetGroupByNameForOrganization = `
SELECT id, organization_id, name, display_name, manager, contact_email, type, parent_id, is_top_group, is_enabled, COALESCE(homepage, '') AS homepage, COALESCE(users, '[]'::jsonb) AS users, created_at, updated_at, revision
FROM guliangiam.groups
WHERE organization_id = guliangiam.resolve_org_id($1) AND name = $2`

// GetByNameForOrganization returns the group with exactly this name in the
// organization, or NotFound. It is an optional capability beyond the domain
// Repository interface; callers type-assert for it and fall back to list scans
// on mirrors that do not implement it.
func (r Repository) GetByNameForOrganization(ctx context.Context, organizationID, name string) (*domain.Group, error) {
	return r.getByIDQuery(ctx, queryGetGroupByNameForOrganization, organizationID, name)
}
