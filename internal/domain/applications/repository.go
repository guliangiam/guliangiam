package applications

import (
	"context"
	"time"
)

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Category       string
	Status         string
}

// HostConflict 是一次跨应用 host 冲突。Host 是归一后的 host[:port]，
// ApplicationID / ApplicationSlug 指向同组织内已占用该 host 的另一个应用。
type HostConflict struct {
	Host            string
	ApplicationID   string
	ApplicationSlug string
}

type Repository interface {
	// GetByID is the system-scoped lookup used by OAuth/OIDC protocol flows
	// (token issuance, logout, QR/WebAuthn login, DCR) where the application is
	// already identified by a previously authenticated token or session. New
	// management routes must use GetByIDForOrganization instead.
	GetByID(ctx context.Context, id string) (*Application, error)
	// GetByIDForOrganization returns the application only when it belongs to
	// organizationID; a foreign or missing row is reported as not found so the
	// two cases stay indistinguishable.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Application, error)
	GetByClientID(ctx context.Context, clientID string) (*Application, error)
	GetByOrganizationAndSlug(ctx context.Context, organizationID string, slug string) (*Application, error)
	List(ctx context.Context, query ListQuery) ([]*Application, int64, error)
	Create(ctx context.Context, application *Application) error
	// Delete removes the application only when it belongs to organizationID;
	// zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
	// Update uses the entity OrganizationID as the tenant predicate, so a
	// cross-tenant write cannot succeed even if a stale id is supplied.
	Update(ctx context.Context, application *Application) error
	ReplaceProviderBindings(ctx context.Context, applicationID string, bindings []ApplicationProviderBinding) error

	// HasDeletedWithOrgAndSlug checks whether a soft-deleted application with
	// the given organization+slug exists. Used to prevent unique-constraint
	// violations when re-creating an application after soft delete.
	HasDeletedWithOrgAndSlug(ctx context.Context, organizationID string, slug string) (bool, error)

	// FindHostConflicts reports applications in the same organization (excluding
	// excludeApplicationID) that already use one of hosts. Callers pass hosts
	// normalized with NormalizeReverseProxyHost; implementations compare both
	// domain and every otherDomains entry with the same normalization, so the
	// check cannot drift from the write-side conflict rules.
	FindHostConflicts(ctx context.Context, organizationID, excludeApplicationID string, hosts []string) ([]HostConflict, error)

	// SetRegistrationAccessToken stores the SHA-256 hash of a dynamically
	// registered client's registration_access_token (RFC 7591 §3.2.1).
	//
	// It is a narrow write on purpose: the applications table carries over a
	// hundred columns, and routing this value through the wide INSERT/UPDATE
	// would require re-aligning every placeholder — exactly the class of change
	// that produced a misaligned-cast defect before (设计报告 14.28).
	SetRegistrationAccessToken(ctx context.Context, applicationID string, tokenHash string) error

	// GetRegistrationAccessTokenHash returns the stored hash for a client, or an
	// empty string when the client is not dynamically registered. Callers must
	// compare it in constant time.
	GetRegistrationAccessTokenHash(ctx context.Context, clientID string) (string, error)

	// CreateClientSecret inserts a new secret row. Callers persist the plaintext
	// in the create/rotate response exactly once; only the hash and a non-secret
	// prefix reach this method. organizationID is the tenant owner and is
	// written into the row so later reads can filter explicitly.
	CreateClientSecret(ctx context.Context, organizationID string, secret *ClientSecret) error

	// ListClientSecrets returns every row for an application ordered newest
	// first. It is the management-read source for clientSecretSet /
	// clientSecretMasked and never exposes hashes to the caller beyond this struct.
	ListClientSecrets(ctx context.Context, organizationID, applicationID string) ([]*ClientSecret, error)

	// ListUsableClientSecrets returns the rows that may verify a presented
	// secret at now: the active row plus grace rows whose grace_until has not
	// elapsed. Rotation expiry is evaluated here, not by a background job.
	ListUsableClientSecrets(ctx context.Context, organizationID, applicationID string, now time.Time) ([]*ClientSecret, error)

	// RotateClientSecret moves the current active row to grace and inserts next
	// as the new active row in one unit of work. The old row is kept (never
	// deleted) so a rotation can be rolled back within the grace window.
	RotateClientSecret(ctx context.Context, organizationID, applicationID string, next *ClientSecret, rotatedAt, graceUntil time.Time) error

	// RetireClientSecrets marks active and grace rows retired at retiredAt.
	// Switching away from a secret-based method keeps the rows for audit and
	// rollback but makes them unusable immediately.
	RetireClientSecrets(ctx context.Context, organizationID, applicationID string, retiredAt time.Time) error

	// ListActiveClientJWKs returns the active public keys registered for a
	// client. It is the only read used by the private_key_jwt verifier, so the
	// tenant predicate is explicit: organizationID is not derived from the
	// presented assertion.
	ListActiveClientJWKs(ctx context.Context, organizationID, applicationID string) ([]*ClientJWK, error)

	// ReplaceClientJWKs retires the client's current active keys and upserts the
	// supplied set in one unit of work. The (application_id, kid) unique index
	// makes a re-registered kid reactivate the existing row instead of creating
	// a duplicate. replacedAt is supplied by the caller so PostgreSQL and MySQL
	// write the same timestamp.
	ReplaceClientJWKs(ctx context.Context, organizationID, applicationID string, keys []*ClientJWK, replacedAt time.Time) error

	// ListActiveClientCertificates returns the active certificate bindings for a
	// client. It is the only read used by the mTLS verifier, so the tenant
	// predicate is explicit and comes from the resolved application row.
	ListActiveClientCertificates(ctx context.Context, organizationID, applicationID string) ([]*ClientCertificate, error)

	// ReplaceClientCertificates retires the client's current active bindings and
	// upserts the supplied set in one unit of work. The (organization_id,
	// fingerprint_sha256) unique index makes a re-uploaded certificate
	// reactivate its existing row; inside one tenant a fingerprint maps to a
	// single application, so the upsert moves the row to applicationID.
	ReplaceClientCertificates(ctx context.Context, organizationID, applicationID string, certificates []*ClientCertificate, replacedAt time.Time) error
}
