package users

import (
	"context"
	"time"
)

// FaceTemplate is the authoritative FaceID login底图 record (one per user).
// TemplateRef carries the decrypted reference material in memory (the base64
// face image captured at enrollment); the infrastructure layer seals it into a
// v2 envelope before persistence so no plaintext image is ever stored at rest.
// It replaces the legacy user_collections.face_ids JSONB plaintext ImageUrl.
type FaceTemplate struct {
	ID             string
	OrganizationID string
	UserID         string
	TemplateRef    string
	Provider       string
	ProviderRef    string
	Status         string
	RevokedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FaceTemplate status values mirror ck_user_face_templates_status.
const (
	FaceTemplateStatusActive  = "active"
	FaceTemplateStatusRevoked = "revoked"
)

// IsActive reports whether the template can be used for FaceID login.
func (t *FaceTemplate) IsActive() bool { return t.Status == FaceTemplateStatusActive }

// FaceTemplateRepository is the storage port for user_face_templates. It is
// exposed only on the concrete UoW through a narrow app-side type assertion
// (like TrustedDevices/RememberMe) so the shared domain UoW interfaces and
// their fakes stay unchanged.
type FaceTemplateRepository interface {
	// Enroll upserts the active template for one user. user_id is UNIQUE, so a
	// re-enrollment updates the existing row in place (重采原位更新) and clears
	// any prior revocation. The implementation seals TemplateRef before writing.
	Enroll(ctx context.Context, template *FaceTemplate) error
	// GetActiveByUser returns the active template only when it belongs to
	// organizationID; a missing/revoked/foreign row is reported as not found.
	// The implementation opens the sealed TemplateRef before returning.
	GetActiveByUser(ctx context.Context, organizationID, userID string) (*FaceTemplate, error)
	// RevokeByUser marks the user's active template revoked (吊销). Zero affected
	// rows is reported as not found so a caller-supplied id cannot cross tenants.
	RevokeByUser(ctx context.Context, organizationID, userID string, revokedAt time.Time) error
}
