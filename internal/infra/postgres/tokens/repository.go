package tokens

import (
	"context"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// refreshTokenConstraints whitelists the constraint identifiers whose names may
// surface in error metadata; anything else stays in the server log.
var refreshTokenConstraints = []string{
	"uq_refresh_tokens_hash",
	"fk_refresh_tokens_access_token_id_access_tokens",
	"fk_refresh_tokens_application_id_applications",
	"fk_refresh_tokens_user_id_users",
	"fk_refresh_tokens_family_id_refresh_tokens",
}

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Token, error) {
	return r.getByID(ctx, queryGetTokenByID, id)
}

// GetByIDForOrganization binds the by-ID read to the trusted tenant through the
// users join. A foreign or missing id is the same ErrNoRows, so callers cannot
// distinguish the two cases.
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Token, error) {
	return r.getByID(ctx, queryGetTokenByIDForOrganization, id, organizationID)
}

func (r Repository) getByID(ctx context.Context, query string, args ...any) (*domain.Token, error) {
	var row tokenRow
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&row.ID, &row.UserID, &row.Username, &row.OrganizationID, &row.OrganizationSlug,
		&row.ApplicationID, &row.ApplicationSlug, &row.TokenPrefix, &row.TokenHash,
		&row.DeviceSecretHash, &row.DeviceSecretExpiresIn,
		&row.Scopes, &row.Audience, &row.Metadata,
		&row.ExpiresAt, &row.RevokedAt, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("token not found")
		}
		return nil, err
	}

	return toToken(row)
}

func (r Repository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Token, error) {
	var row tokenRow
	err := r.db.QueryRow(ctx, queryGetTokenByHash, tokenHash).Scan(
		&row.ID, &row.UserID, &row.Username, &row.OrganizationID, &row.OrganizationSlug,
		&row.ApplicationID, &row.ApplicationSlug, &row.TokenPrefix, &row.TokenHash,
		&row.DeviceSecretHash, &row.DeviceSecretExpiresIn,
		&row.Scopes, &row.Audience, &row.Metadata,
		&row.ExpiresAt, &row.RevokedAt, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("token not found")
		}
		return nil, err
	}

	return toToken(row)
}

func (r Repository) GetByDeviceSecretHash(ctx context.Context, deviceSecretHash string) (*domain.Token, error) {
	var row tokenRow
	err := r.db.QueryRow(ctx, queryGetTokenByDeviceSecretHash, deviceSecretHash).Scan(
		&row.ID, &row.UserID, &row.Username, &row.OrganizationID, &row.OrganizationSlug,
		&row.ApplicationID, &row.ApplicationSlug, &row.TokenPrefix, &row.TokenHash,
		&row.DeviceSecretHash, &row.DeviceSecretExpiresIn,
		&row.Scopes, &row.Audience, &row.Metadata,
		&row.ExpiresAt, &row.RevokedAt, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("token not found by device secret")
		}
		return nil, err
	}

	return toToken(row)
}

func (r Repository) GetRefreshByTokenHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	var row refreshTokenRow
	err := r.db.QueryRow(ctx, queryGetRefreshTokenByHash, tokenHash).Scan(
		&row.ID,
		&row.AccessTokenID,
		&row.UserID,
		&row.Username,
		&row.OrganizationID,
		&row.OrganizationSlug,
		&row.ApplicationID,
		&row.ApplicationSlug,
		&row.RefreshTokenHash,
		&row.FamilyID,
		&row.Scopes,
		&row.Audience,
		&row.ExpiresAt,
		&row.RevokedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
		&row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("refresh token not found")
		}
		return nil, err
	}

	return toRefreshToken(row), nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Token, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListTokens, query.OrganizationID, query.UserID, query.ApplicationID, query.ActiveOnly, query.Keyword, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Token, 0)
	for rows.Next() {
		var row tokenRow
		if err := rows.Scan(
			&row.ID, &row.UserID, &row.Username, &row.OrganizationID, &row.OrganizationSlug,
			&row.ApplicationID, &row.ApplicationSlug, &row.TokenPrefix, &row.TokenHash,
			&row.DeviceSecretHash, &row.DeviceSecretExpiresIn,
			&row.Scopes, &row.Audience, &row.Metadata,
			&row.ExpiresAt, &row.RevokedAt, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}

		token, err := toToken(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode token")
		}
		items = append(items, token)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountTokens, query.OrganizationID, query.UserID, query.ApplicationID, query.ActiveOnly, query.Keyword).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, token *domain.Token) error {
	metadata, err := marshalJSONObject(token.Metadata)
	if err != nil {
		return apperrors.NewInternal("encode token metadata")
	}

	_, err = r.db.Exec(ctx, queryInsertToken,
		token.ID, token.UserID, token.ApplicationID, token.TokenHash, token.TokenPrefix,
		nullableString(token.DeviceSecretHash), token.DeviceSecretExpiresIn,
		nonNilStrings(token.Scopes), nonNilStrings(token.Audience), metadata,
		token.ExpiresAt, token.RevokedAt, token.CreatedAt, token.UpdatedAt, token.Revision,
	)
	return err
}

// UpdateDeviceSecret writes the hashed Native SSO device secret onto an access
// token that has already been inserted.
func (r Repository) UpdateDeviceSecret(ctx context.Context, tokenID, deviceSecretHash string, expiresIn int) error {
	if tokenID == "" || deviceSecretHash == "" {
		return apperrors.NewInternal("update device secret: token id and hash are required")
	}
	tag, err := r.db.Exec(ctx, queryUpdateTokenDeviceSecret, tokenID, deviceSecretHash, expiresIn)
	if err != nil {
		return apperrors.WrapInternal(err, "update device secret")
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("token not found")
	}
	return nil
}

func (r Repository) CreateRefresh(ctx context.Context, token *domain.RefreshToken) error {
	_, err := r.db.Exec(ctx, queryInsertRefreshToken,
		token.ID, nullableString(token.AccessTokenID), token.UserID, token.ApplicationID,
		token.RefreshTokenHash, "", nullableString(token.FamilyID),
		nonNilStrings(token.Scopes), nonNilStrings(token.Audience),
		token.ExpiresAt, token.RevokedAt, token.CreatedAt, token.UpdatedAt, token.Revision,
	)
	return pgerror.Map(err, refreshTokenConstraints...)
}

func (r Repository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	result, err := r.db.Exec(ctx, queryRevokeToken, id, revokedAt)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		token, getErr := r.GetByID(ctx, id)
		if getErr != nil {
			return getErr
		}
		if token.RevokedAt != nil {
			return nil
		}
	}

	return nil
}

// RevokeRefresh revokes a refresh token and reports whether this call was the one
// that changed it. A caller inside a rotation transaction must treat
// ErrRefreshAlreadyRevoked as token reuse: another request already rotated the
// pair, so issuing a second pair would hand out two live sessions.
func (r Repository) RevokeRefresh(ctx context.Context, id string, revokedAt time.Time) error {
	result, err := r.db.Exec(ctx, queryRevokeRefreshToken, id, revokedAt)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return domain.ErrRefreshAlreadyRevoked
	}
	return nil
}

func (r Repository) RevokeByQuery(ctx context.Context, query domain.RevokeQuery, revokedAt time.Time) (int64, error) {
	result, err := r.db.Exec(ctx, queryRevokeTokensByQuery, query.OrganizationID, query.UserID, query.ApplicationID, revokedAt, query.TokenPrefix)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// RevokeByFamilyID revokes every refresh token in the family together with the
// access tokens they were paired with, and reports how many refresh tokens were
// revoked. Returning the refresh count (rather than the access-token count) is
// what the reuse detector logs as tokens_revoked.
func (r Repository) RevokeByFamilyID(ctx context.Context, familyID string, revokedAt time.Time) (int64, error) {
	var affected int64
	if err := r.db.QueryRow(ctx, queryRevokeByFamilyID, familyID, revokedAt).Scan(&affected); err != nil {
		return 0, err
	}

	return affected, nil
}

func (r Repository) RevokeRefreshByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	var count int64
	if err := r.db.QueryRow(ctx, queryRevokeRefreshByUser, userID, revokedAt).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.Exec(ctx, queryDeleteToken, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewNotFound("token not found")
	}
	return nil
}

// RevokeByAuthorizationCodeID revokes the access token (and its paired refresh
// token) that the given authorization code produced. It reports how many rows
// were revoked so the caller can log the blast radius of a leaked code.
func (r Repository) RevokeByAuthorizationCodeID(ctx context.Context, authorizationCodeID string, revokedAt time.Time) (int64, error) {
	var affected int64
	if err := r.db.QueryRow(ctx, queryRevokeByAuthorizationCodeID, authorizationCodeID, revokedAt).Scan(&affected); err != nil {
		return 0, err
	}

	return affected, nil
}

func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func marshalJSONObject(value map[string]any) ([]byte, error) {
	if value == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(value)
}

// nonNilStrings keeps a nil slice out of the wire format so pgx encodes it as an
// empty text[] (the columns are NOT NULL DEFAULT '{}').
func nonNilStrings(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}

var _ domain.Repository = Repository{}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
