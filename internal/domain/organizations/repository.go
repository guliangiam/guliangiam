package organizations

import "context"

type ListQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
}

// SessionPolicyReader is the optional narrow read used on the authentication
// hot path. Resolving a session only needs the organization's session policy
// (idle timeout + client-binding flags), not the full aggregate: the aggregate
// query joins eight tables and decrypts secret columns. Implementations that
// provide this method let the auth path read one column from one row.
//
// It is deliberately separate from Repository so existing fakes and the
// remaining implementations keep compiling; callers fall back to GetByID when
// the capability is absent.
type SessionPolicyReader interface {
	// GetSessionPolicy returns the organization's session_policy JSON object.
	// A missing policy row is not an error and yields an empty map.
	GetSessionPolicy(ctx context.Context, organizationID string) (map[string]any, error)
}

type Repository interface {
	GetByID(ctx context.Context, id string) (*Organization, error)
	GetBySlug(ctx context.Context, slug string) (*Organization, error)
	// GetBySlugPublic bypasses tenant RLS — used by public discovery endpoints
	// (/by-slug, /resolve) that have no session context. Implementations that
	// lack row-level security (MySQL) may simply delegate to GetBySlug.
	GetBySlugPublic(ctx context.Context, slug string) (*Organization, error)
	List(ctx context.Context, query ListQuery) ([]*Organization, int64, error)
	Create(ctx context.Context, organization *Organization) error
	Update(ctx context.Context, organization *Organization) error
	UpdateSettings(ctx context.Context, settings *OrganizationSettings) error
	UpdateTheme(ctx context.Context, theme *OrganizationThemeSettings) error
	UpdateSecurityPolicy(ctx context.Context, policy *OrganizationSecurityPolicy) error
	Delete(ctx context.Context, id string) error

	// HasDeletedWithSlug checks whether a soft-deleted organization with the
	// given slug exists. This is used to prevent unique-constraint violations
	// when re-creating a resource with the same name after soft delete.
	HasDeletedWithSlug(ctx context.Context, slug string) (bool, error)
}
