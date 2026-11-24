package trusteddevices

import (
	"context"
	"time"
)

// Repository persists trusted-device rows. Every tenant-scoped query filters
// organization_id explicitly (MySQL has no RLS; PostgreSQL RLS is defence in
// depth, not the only guard).
type Repository interface {
	Create(ctx context.Context, device *TrustedDevice) error
	// GetByID returns NotFound when the row does not exist. The caller must
	// still verify user/organization ownership before acting on it.
	GetByID(ctx context.Context, id string) (*TrustedDevice, error)
	ListByUser(ctx context.Context, organizationID, userID string) ([]*TrustedDevice, error)
	// FindActiveByCredential returns the row matching credential + UA hash that
	// is neither revoked nor expired. Not-found is reported as NotFound so the
	// caller can fall through to the normal MFA flow.
	FindActiveByCredential(ctx context.Context, organizationID, userID, credentialID, uaHash string, now time.Time) (*TrustedDevice, error)
	// Revoke is idempotent: revoking an already revoked row is not an error.
	Revoke(ctx context.Context, organizationID, id string, now time.Time) error
	TouchLastSeen(ctx context.Context, id string, now time.Time) error
	// RevokeByCredential revokes any still-active row sharing a credential so a
	// re-registration cannot leave two live rows for the same device.
	RevokeByCredential(ctx context.Context, organizationID, userID, credentialID string, now time.Time) error
	// RevokeByUser 批量吊销该用户在该组织下所有仍有效的受信设备（密码重置等
	// 凭据级联场景）。返回本次实际吊销的行数；重复调用返回 0，不是错误。
	RevokeByUser(ctx context.Context, organizationID, userID string, now time.Time) (int64, error)
}

// ExpiredCleaner is implemented by repositories that can physically delete
// expired rows in bounded batches. It is optional so existing fakes stay valid.
type ExpiredCleaner interface {
	DeleteExpired(ctx context.Context, before time.Time, limit int) (int64, error)
}

// LegacyPlaintextRevoker is implemented by repositories that can revoke rows
// whose device_credential_id is not a SHA-256 digest. Those rows predate the
// digest switch and can never match FindActiveByCredential, so they are revoked
// explicitly instead of being left to fail closed silently (OPEN-API-49).
type LegacyPlaintextRevoker interface {
	RevokeLegacyPlaintext(ctx context.Context, now time.Time) (int64, error)
}
