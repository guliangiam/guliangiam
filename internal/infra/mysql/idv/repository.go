package idv

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"

	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// identityVerificationEncryptedMapper 负责 session_token 列的信封加解密（A-6.2）。
// 列定义保持 VARCHAR(255)，库内改为绑定 (table, column, 行 id) 的 v2 信封。
var identityVerificationEncryptedMapper = mysqlshared.NewEncryptedFieldMapper()

const (
	identityVerificationTable              = "guliangiam.identity_verifications"
	identityVerificationColumnSessionToken = "session_token"
	identityVerificationSessionTokenMaxLen = 255
)

func identityVerificationAAD(verificationID string) crypto.AADContext {
	return crypto.AADContext{Table: identityVerificationTable, Column: identityVerificationColumnSessionToken, PKValue: verificationID}
}

// openSessionToken 解密信封。非信封值按 A-6.2 之前的存量明文兼容读取，
// 但必须写结构化 WARN；WARN 只包含行标识，不含明文或密文。
func openSessionToken(verificationID, organizationID string, stored *string) (*string, error) {
	if stored == nil || *stored == "" {
		return stored, nil
	}

	plaintext, err := identityVerificationEncryptedMapper.DecryptEnvelopeColumn(identityVerificationAAD(verificationID), *stored)
	if err == nil {
		return &plaintext, nil
	}
	if !errors.Is(err, crypto.ErrNotEnvelope) {
		return nil, apperrors.WrapInternalf(err, "decrypt identity verification session_token")
	}

	if legacy, legacyErr := identityVerificationEncryptedMapper.DecryptColumn(identityVerificationAAD(verificationID), *stored); legacyErr == nil {
		slog.Warn("identity verification session_token is a legacy v0 envelope; run `migrate encrypt-credential-secrets`",
			"verification_id", verificationID, "organization_id", organizationID)
		return &legacy, nil
	}

	slog.Warn("identity verification session_token is stored in plaintext; run `migrate encrypt-credential-secrets`",
		"verification_id", verificationID, "organization_id", organizationID)
	return stored, nil
}

// sealSessionToken 返回持久化值，不改 domain 对象；nil 与空值保持空值。
func sealSessionToken(verificationID string, token *string) (*string, error) {
	if token == nil || *token == "" {
		return token, nil
	}
	sealed, err := identityVerificationEncryptedMapper.EncryptColumn(identityVerificationAAD(verificationID), *token)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "encrypt identity verification session_token")
	}
	if len(sealed) > identityVerificationSessionTokenMaxLen {
		return nil, apperrors.NewInternalf(
			"encrypted identity verification session_token is %d characters, exceeding the %d-character column",
			len(sealed), identityVerificationSessionTokenMaxLen)
	}
	return &sealed, nil
}

// Repository 是 MySQL 8.4+ 实现的 idv 存储层。
// 与 internal/infra/postgres/idv.Repository 实现同域接口。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// scannable 是 sql.Row 和 sql.Rows 的公共接口。
type scannable interface {
	Scan(dest ...any) error
}

// scanOne 扫描 identity_verifications 表，21 列严格匹配列顺序。
func scanOne(s scannable) (*domain.Verification, error) {
	var v domain.Verification
	var raw json.RawMessage
	var storedToken *string
	err := s.Scan(
		&v.ID, &v.OrganizationID, &v.UserID, &v.Provider, &v.Status,
		&v.RedirectURL, &storedToken, &v.ProviderRef,
		&v.FirstName, &v.LastName, &v.DateOfBirth, &v.Nationality,
		&v.IDType, &v.IDNumber, &v.Confidence, &v.Reason,
		&raw, &v.InitiatedAt, &v.CompletedAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("verification not found")
		}
		return nil, err
	}
	token, err := openSessionToken(v.ID, v.OrganizationID, storedToken)
	if err != nil {
		return nil, err
	}
	v.SessionToken = token
	v.RawResult = raw
	return &v, nil
}

// ── domain.Repository 实现 ────────────────────────────────────────

// Create 插入 verification，session_token 写入前封装。
func (r Repository) Create(ctx context.Context, v *domain.Verification) error {
	raw := v.RawResult
	if raw == nil {
		raw = json.RawMessage(`{}`)
	}
	sessionToken, err := sealSessionToken(v.ID, v.SessionToken)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsert,
		v.ID, v.OrganizationID, v.UserID, v.Provider, v.Status,
		v.RedirectURL, sessionToken, v.ProviderRef,
		v.FirstName, v.LastName, v.DateOfBirth, v.Nationality,
		v.IDType, v.IDNumber, v.Confidence, v.Reason,
		raw, v.InitiatedAt, v.CompletedAt, v.CreatedAt, v.UpdatedAt,
	)
	return err
}

// Update 更新 verification。
func (r Repository) Update(ctx context.Context, v *domain.Verification) error {
	raw := v.RawResult
	if raw == nil {
		raw = json.RawMessage(`{}`)
	}
	_, err := r.db.ExecContext(ctx, queryUpdate,
		v.Status,
		v.FirstName, v.LastName, v.DateOfBirth, v.Nationality,
		v.IDType, v.IDNumber, v.Confidence, v.Reason,
		raw, v.CompletedAt,
		v.ProviderRef, v.UpdatedAt,
		v.ID, v.OrganizationID,
	)
	return err
}

// GetByID 按 id + organization_id 取 verification。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Verification, error) {
	return scanOne(r.db.QueryRowContext(ctx, queryGetByID, id, organizationID))
}

// GetByProviderRef 按 provider + provider_ref 取 verification。
func (r Repository) GetByProviderRef(ctx context.Context, provider, providerRef string) (*domain.Verification, error) {
	return scanOne(r.db.QueryRowContext(ctx, queryGetByProviderRef, provider, providerRef))
}

// List 分页列出 verifications。
// 参数顺序：orgID, userFilter×2, providerFilter×2, statusFilter×2, limit, offset
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Verification, int64, error) {
	userFilter := ""
	providerFilter := ""
	statusFilter := (*string)(nil)
	if q.UserID != nil {
		userFilter = *q.UserID
	}
	if q.Provider != nil {
		providerFilter = *q.Provider
	}
	if q.Status != nil {
		s := string(*q.Status)
		statusFilter = &s
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)

	rows, err := r.db.QueryContext(ctx, queryList,
		q.OrganizationID,
		userFilter, userFilter,
		providerFilter, providerFilter,
		statusFilter, statusFilter,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Verification, 0, limit)
	for rows.Next() {
		v, err := scanOne(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, queryCount,
		q.OrganizationID,
		userFilter, userFilter,
		providerFilter, providerFilter,
		statusFilter, statusFilter,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Delete 按 id + organization_id 删除。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	_, err := r.db.ExecContext(ctx, queryDelete, id, organizationID)
	return err
}

// 编译期接口断言
var _ domain.Repository = Repository{}
