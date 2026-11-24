package users

import "context"

type ListQuery struct {
	Page     int
	PageSize int
	// Offset, when > 0, overrides the page-derived row offset. SCIM
	// startIndex is a 1-based row offset, not a page number (BE-E-07).
	// Mirrors that cannot honor it may ignore it.
	Offset         int
	OrganizationID string
	Keyword        string
	Status         string
	UserType       string
	// IsAdmin filters on users.is_admin when non-nil. Used by the risk engine
	// to enumerate tenant administrators for critical security alerts without
	// paging through the whole organization.
	IsAdmin     *bool
	SortField   string
	SortOrder   string
	SearchField string
	SearchValue string
	// Lean requests a list-only projection: implementations may skip loading
	// aggregates that list/export consumers never read (credentials,
	// identities, MFA methods) while still returning the main row and profile.
	// Callers that need those aggregates (SCIM, social login, search, LDAP,
	// risk) must leave it false. Mirrors that cannot honor it may ignore it.
	Lean bool
}

type Repository interface {
	// GetByID is the system-scoped lookup used by authentication, token refresh
	// and background sync, which run without a request tenant. Management and
	// SCIM by-ID routes must use GetByIDForOrganization so a caller-supplied id
	// cannot cross tenants.
	GetByID(ctx context.Context, id string) (*User, error)
	// GetByIDForOrganization returns the user only when it belongs to
	// organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*User, error)
	// LockByID 在同一事务内对该用户行加行级锁（FOR UPDATE），持续到事务提交或回滚。
	//
	// 用途是给「登录后的会话记账」提供互斥锚点：先锁用户行，再读会话数并淘汰，否则两个
	// 并发登录都会读到「未超限」而同时放行（《开发设计》22.5 要求计数与淘汰在同一 user_id
	// 的锁内完成）。锚点必须是用户行而不是会话行——首次登录时会话行尚不存在，锁一个不存在
	// 的行不会互斥。未持有事务的连接上调用它没有意义（行锁随语句结束释放）。
	LockByID(ctx context.Context, id string) error
	GetByUsername(ctx context.Context, organizationID string, username string) (*User, error)
	// GetByLoginIdentifier resolves one account identifier (username, email or
	// phone) inside organizationID. Precedence is username (exact) > email
	// (case-insensitive) > phone (exact). When the identifier matches more than
	// one distinct user — possible only for phone, which has no unique
	// constraint — the lookup fails closed as not found so a logged-in account
	// cannot be chosen by ambiguity. Callers use this for password login only;
	// every other consumer keeps using GetByUsername.
	GetByLoginIdentifier(ctx context.Context, organizationID string, identifier string) (*User, error)
	List(ctx context.Context, query ListQuery) ([]*User, int64, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	// UpdateForOrganization persists the user aggregate only when it belongs to
	// organizationID; zero affected rows is reported as not found.
	UpdateForOrganization(ctx context.Context, user *User, organizationID string) error
	UpdateProfile(ctx context.Context, profile *UserProfile) error
	UpdateCredentials(ctx context.Context, credentials *UserCredentials) error
	// UpdatePasswordCredential updates ONLY password_hash, password_algo,
	// password_updated_at and need_update_password. Unlike UpdateCredentials
	// (full-row upsert) it never touches access_key/access_secret_hash, so a
	// password-only change cannot wipe an existing access secret.
	UpdatePasswordCredential(ctx context.Context, credentials *UserCredentials) error
	// UpdateCollectionsForOrganization replaces the JSONB collection rows of a
	// user that belongs to organizationID. Zero affected rows is reported as
	// not found so a caller-supplied id cannot cross tenants.
	UpdateCollectionsForOrganization(ctx context.Context, userID, organizationID string, collections UserCollections) error
	// SetContactVerifiedForOrganization flips the verified flags of a profile
	// only when the user still belongs to organizationID.
	SetContactVerifiedForOrganization(ctx context.Context, userID, organizationID string, emailVerified, phoneVerified bool) error
	ReplaceIdentities(ctx context.Context, userID string, identities []UserIdentity) error
	CreateIdentity(ctx context.Context, identity *UserIdentity) error
	DeleteIdentity(ctx context.Context, userID string, identityID string) error
	ReplaceMfaMethods(ctx context.Context, userID string, methods []UserMfaMethod) error
	ReplaceRoleBindings(ctx context.Context, userID string, bindings []UserRoleBinding) error
	// ListRoleNames returns the Role.Name values of every role the given user is bound to.
	// Used by the auth service to derive frontend permission sets from Casbin rules.
	ListRoleNames(ctx context.Context, userID string) ([]string, error)
	ReplaceGroupMemberships(ctx context.Context, userID string, memberships []UserGroupMembership) error
	// Delete soft-deletes the user only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error

	// Password history
	GetPasswordHistory(ctx context.Context, userID string, limit int) ([]PasswordHistoryEntry, error)
	CreatePasswordHistory(ctx context.Context, entry *PasswordHistoryEntry) error
	DeletePasswordHistoryBeyond(ctx context.Context, userID string, keepCount int) error

	// Social login: find user by third-party identity
	GetByProviderIdentity(ctx context.Context, providerID string, subject string) (*User, error)

	// Count users, optionally filtered by organization
	Count(ctx context.Context, organizationID string) (int64, error)

	// HasDeletedWithUsername checks whether a soft-deleted user with the given
	// username exists in the specified organization. Used to prevent unique-
	// constraint violations when re-creating a user after soft delete.
	HasDeletedWithUsername(ctx context.Context, organizationID string, username string) (bool, error)
}
