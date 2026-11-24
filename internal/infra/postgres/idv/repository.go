package idv

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// identityVerificationEncryptedMapper guards session_token (A-6.2). The column
// keeps its historical VARCHAR(255) definition and now stores a v2 envelope
// bound to (table, column, row id) instead of the plaintext provider session.
var identityVerificationEncryptedMapper = shared.NewEncryptedFieldMapper()

const (
	identityVerificationTable              = "guliangiam.identity_verifications"
	identityVerificationColumnSessionToken = "session_token"
	identityVerificationColumnIDNumber     = "id_number"
	identityVerificationColumnRawResult    = "raw_result"
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
	// The column is still VARCHAR(255). Refuse an oversized envelope instead of
	// letting the database silently fail the write; no value is logged.
	if len(sealed) > identityVerificationSessionTokenMaxLen {
		return nil, apperrors.NewInternalf(
			"encrypted identity verification session_token is %d characters, exceeding the %d-character column",
			len(sealed), identityVerificationSessionTokenMaxLen)
	}
	return &sealed, nil
}

// identityVerificationSealedColumn resolves the A-6.3B mapping for one legacy
// plaintext column. The target column and PK come from the registry so the
// write/read AAD cannot drift from `migrate backfill-pii-sealed-columns`.
func identityVerificationSealedColumn(sourceColumn string) (crypto.LegacySealedColumn, error) {
	for _, col := range crypto.LegacySealedColumns {
		if col.Table == identityVerificationTable && col.SourceColumn == sourceColumn {
			return col, nil
		}
	}
	return crypto.LegacySealedColumn{}, apperrors.NewInternalf(
		"no sealed target registered for %s.%s", identityVerificationTable, sourceColumn)
}

// sealIdentityVerificationPII writes the v2 twin of one legacy PII column.
// NULL and empty values stay unsealed, so the target column keeps NULL
// semantics ("legacy plaintext row remains").
func sealIdentityVerificationPII(sourceColumn, verificationID string, plaintext *string) (*string, error) {
	if plaintext == nil || *plaintext == "" {
		return nil, nil
	}
	col, err := identityVerificationSealedColumn(sourceColumn)
	if err != nil {
		return nil, err
	}
	sealed, err := identityVerificationEncryptedMapper.EncryptColumn(col.AAD(verificationID), *plaintext)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "encrypt identity verification %s", col.TargetColumn)
	}
	return &sealed, nil
}

// openIdentityVerificationPII prefers the sealed target. A NULL/empty target is
// the upgrade window: the legacy plaintext column is returned and a WARN marks
// the row for the backfill. A sealed value that cannot be authenticated is an
// error and never silently downgrades to plaintext.
func openIdentityVerificationPII(sourceColumn, verificationID, organizationID string, plaintext, sealed *string) (*string, error) {
	if sealed == nil || *sealed == "" {
		if plaintext != nil && *plaintext != "" {
			slog.Warn("identity verification PII column is not sealed; run `migrate backfill-pii-sealed-columns`",
				"verification_id", verificationID, "organization_id", organizationID, "column", sourceColumn)
		}
		return plaintext, nil
	}
	col, err := identityVerificationSealedColumn(sourceColumn)
	if err != nil {
		return nil, err
	}
	opened, err := identityVerificationEncryptedMapper.DecryptEnvelopeColumn(col.AAD(verificationID), *sealed)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decrypt identity verification %s", col.TargetColumn)
	}
	return &opened, nil
}

// sealRawResult seals the JSON text of raw_result. The plaintext column always
// receives a JSON object, so a nil RawResult is normalized by the caller before
// this point.
func sealRawResult(verificationID string, raw json.RawMessage) (*string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	plaintext := string(raw)
	return sealIdentityVerificationPII(identityVerificationColumnRawResult, verificationID, &plaintext)
}

// openRawResult prefers raw_result_sealed and validates that the decrypted
// text is still JSON before it reaches the domain object.
func openRawResult(verificationID, organizationID string, raw json.RawMessage, sealed *string) (json.RawMessage, error) {
	plaintext := string(raw)
	opened, err := openIdentityVerificationPII(identityVerificationColumnRawResult, verificationID, organizationID, &plaintext, sealed)
	if err != nil {
		return nil, err
	}
	if opened == nil || *opened == "" {
		return nil, nil
	}
	if !json.Valid([]byte(*opened)) {
		return nil, apperrors.NewInternal("identity verification raw_result_sealed decrypted to invalid JSON")
	}
	return json.RawMessage(*opened), nil
}

// Repository is the postgres-backed implementation of domain.Repository.
type Repository struct{ db shared.DBTX }

// NewRepository constructs a Repository bound to the given DBTX.
func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

var _ domain.Repository = Repository{}

// ─── Create ──────────────────────────────────────────────────────────────────

func (r Repository) Create(ctx context.Context, v *domain.Verification) error {
	var raw json.RawMessage
	if v.RawResult != nil {
		raw = v.RawResult
	} else {
		raw = json.RawMessage(`{}`)
	}
	sessionToken, err := sealSessionToken(v.ID, v.SessionToken)
	if err != nil {
		return err
	}
	idNumberSealed, err := sealIdentityVerificationPII(identityVerificationColumnIDNumber, v.ID, v.IDNumber)
	if err != nil {
		return err
	}
	rawResultSealed, err := sealRawResult(v.ID, raw)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryInsert,
		v.ID, v.OrganizationID, v.UserID, v.Provider, v.Status,
		v.RedirectURL, sessionToken, v.ProviderRef,
		v.FirstName, v.LastName, v.DateOfBirth, v.Nationality,
		v.IDType, v.IDNumber, idNumberSealed, v.Confidence, v.Reason,
		raw, rawResultSealed, v.InitiatedAt, v.CompletedAt, v.CreatedAt, v.UpdatedAt,
		v.BizSeq, v.Action, v.ProductCode, v.SubjectRef,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_idv_biz_seq" {
			return apperrors.NewConflict("verification biz_seq has already been consumed")
		}
	}
	return err
}

// ─── Update ──────────────────────────────────────────────────────────────────

func (r Repository) Update(ctx context.Context, v *domain.Verification) error {
	var raw json.RawMessage
	if v.RawResult != nil {
		raw = v.RawResult
	} else {
		raw = json.RawMessage(`{}`)
	}
	idNumberSealed, err := sealIdentityVerificationPII(identityVerificationColumnIDNumber, v.ID, v.IDNumber)
	if err != nil {
		return err
	}
	rawResultSealed, err := sealRawResult(v.ID, raw)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryUpdate,
		v.Status,
		v.FirstName, v.LastName, v.DateOfBirth, v.Nationality,
		v.IDType, v.IDNumber, idNumberSealed, v.Confidence, v.Reason,
		raw, rawResultSealed, v.CompletedAt,
		v.ProviderRef, v.UpdatedAt,
		v.ID, v.OrganizationID,
		v.SubjectRef,
	)
	return err
}

// ─── GetByID ────────────────────────────────────────────────────────────────

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Verification, error) {
	return scanOne(r.db.QueryRow(ctx, queryGetByID, id, organizationID))
}

// ─── GetByProviderRef ───────────────────────────────────────────────────────

func (r Repository) GetByProviderRef(ctx context.Context, provider, providerRef string) (*domain.Verification, error) {
	return scanOne(r.db.QueryRow(ctx, queryGetByProviderRef, provider, providerRef))
}

// ─── List ────────────────────────────────────────────────────────────────────

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
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, queryList,
		q.OrganizationID, userFilter, providerFilter, statusFilter,
		shared.ClampPageSize(q.PageSize), offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Verification, 0, q.PageSize)
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
	if err := r.db.QueryRow(ctx, queryCount,
		q.OrganizationID, userFilter, providerFilter, statusFilter,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	_, err := r.db.Exec(ctx, queryDelete, id, organizationID)
	return err
}

// ─── scan helper ─────────────────────────────────────────────────────────────

func scanOne(row pgx.Row) (*domain.Verification, error) {
	var v domain.Verification
	var raw json.RawMessage
	var storedToken, idNumberSealed, rawResultSealed *string
	err := row.Scan(
		&v.ID, &v.OrganizationID, &v.UserID, &v.Provider, &v.Status,
		&v.RedirectURL, &storedToken, &v.ProviderRef,
		&v.BizSeq, &v.Action, &v.ProductCode, &v.SubjectRef,
		&v.FirstName, &v.LastName, &v.DateOfBirth, &v.Nationality,
		&v.IDType, &v.IDNumber, &idNumberSealed, &v.Confidence, &v.Reason,
		&raw, &rawResultSealed, &v.InitiatedAt, &v.CompletedAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("verification not found")
		}
		return nil, err
	}
	token, err := openSessionToken(v.ID, v.OrganizationID, storedToken)
	if err != nil {
		return nil, err
	}
	idNumber, err := openIdentityVerificationPII(identityVerificationColumnIDNumber, v.ID, v.OrganizationID, v.IDNumber, idNumberSealed)
	if err != nil {
		return nil, err
	}
	rawResult, err := openRawResult(v.ID, v.OrganizationID, raw, rawResultSealed)
	if err != nil {
		return nil, err
	}
	v.SessionToken = token
	v.IDNumber = idNumber
	v.RawResult = rawResult
	return &v, nil
}
