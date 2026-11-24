// Package facetemplates 是 user_face_templates 的 MySQL 8.4+ 镜像存储层，
// 与 internal/infra/postgres/facetemplates 实现同一 domain 端口。template_ref
// 以绑定 (table, column, user_id) 的 v2 信封密文落库，绝不存明文人脸底图。
package facetemplates

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	faceTemplateTable     = "guliangiam.user_face_templates"
	faceTemplateColumnRef = "template_ref"
)

var faceTemplateEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

func faceTemplateAAD(userID string) crypto.AADContext {
	return crypto.AADContext{Table: faceTemplateTable, Column: faceTemplateColumnRef, PKValue: userID}
}

// Repository 是 MySQL 实现的 face template 存储层。
type Repository struct{ db mysqlshared.DBTX }

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository { return Repository{db: db} }

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
	_, err = r.db.ExecContext(ctx, queryEnroll,
		template.ID, template.OrganizationID, template.UserID, sealed,
		nullableString(template.Provider), nullableString(template.ProviderRef),
		template.CreatedAt, now,
	)
	return err
}

func (r Repository) GetActiveByUser(ctx context.Context, organizationID, userID string) (*domain.FaceTemplate, error) {
	var t domain.FaceTemplate
	var sealed, provider, providerRef *string
	err := r.db.QueryRowContext(ctx, queryGetActiveByUser, organizationID, userID).Scan(
		&t.ID, &t.OrganizationID, &t.UserID, &sealed, &provider, &providerRef,
		&t.Status, &t.RevokedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
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

func (r Repository) RevokeByUser(ctx context.Context, organizationID, userID string, revokedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, queryRevokeByUser, revokedAt, revokedAt, organizationID, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("no active face template for user")
	}
	return nil
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
