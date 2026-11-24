// Package facetemplates persists user_face_templates: the authoritative FaceID
// login底图 store. template_ref is sealed into a v2 envelope bound to
// (table, column, user_id) so no plaintext face image is ever written at rest.
package facetemplates

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

const (
	faceTemplateTable     = "guliangiam.user_face_templates"
	faceTemplateColumnRef = "template_ref"
)

var faceTemplateEncryptedMapper = shared.NewEncryptedFieldMapper()

// faceTemplateAAD binds the envelope to the user_id unique key. user_id is
// UNIQUE (one template per user) and stable across re-enrollment upserts, so
// the same binding authenticates the ciphertext before and after 原位更新.
func faceTemplateAAD(userID string) crypto.AADContext {
	return crypto.AADContext{Table: faceTemplateTable, Column: faceTemplateColumnRef, PKValue: userID}
}

// Repository is the postgres-backed implementation of domain.FaceTemplateRepository.
type Repository struct{ db shared.DBTX }

// NewRepository constructs a Repository bound to the given DBTX.
func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

var _ domain.FaceTemplateRepository = Repository{}

func (r Repository) Enroll(ctx context.Context, template *domain.FaceTemplate) error {
	if template.OrganizationID == "" || template.UserID == "" || template.ID == "" {
		return apperrors.NewInvalidArgument("face template id, organizationId and userId are required")
	}
	if template.TemplateRef == "" {
		return apperrors.NewInvalidArgument("face template reference is required")
	}
	sealed, err := faceTemplateEncryptedMapper.EncryptColumn(faceTemplateAAD(template.UserID), template.TemplateRef)
	if err != nil {
		return apperrors.WrapInternalf(err, "encrypt face template reference")
	}
	now := template.UpdatedAt
	if now.IsZero() {
		now = template.CreatedAt
	}
	_, err = r.db.Exec(ctx, queryEnroll,
		template.ID, template.OrganizationID, template.UserID, sealed,
		nullableString(template.Provider), nullableString(template.ProviderRef),
		template.CreatedAt, now,
	)
	return err
}

func (r Repository) GetActiveByUser(ctx context.Context, organizationID, userID string) (*domain.FaceTemplate, error) {
	return scanOne(r.db.QueryRow(ctx, queryGetActiveByUser, organizationID, userID))
}

func (r Repository) RevokeByUser(ctx context.Context, organizationID, userID string, revokedAt time.Time) error {
	tag, err := r.db.Exec(ctx, queryRevokeByUser, organizationID, userID, revokedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("no active face template for user")
	}
	return nil
}

func scanOne(row pgx.Row) (*domain.FaceTemplate, error) {
	var t domain.FaceTemplate
	var sealed, provider, providerRef *string
	err := row.Scan(
		&t.ID, &t.OrganizationID, &t.UserID, &sealed, &provider, &providerRef,
		&t.Status, &t.RevokedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("no active face template for user")
		}
		return nil, err
	}
	if sealed == nil || *sealed == "" {
		return nil, apperrors.NewInternal("face template reference is empty")
	}
	opened, err := faceTemplateEncryptedMapper.DecryptEnvelopeColumn(faceTemplateAAD(t.UserID), *sealed)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt face template reference")
	}
	t.TemplateRef = opened
	t.Provider = derefString(provider)
	t.ProviderRef = derefString(providerRef)
	return &t, nil
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
