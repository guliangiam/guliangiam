package authorizationcodes

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// codePrefixLength is how many leading characters of the plaintext code are
// kept for troubleshooting. It is stored but never used for matching: the
// lookup always goes through the SHA-256 hash.
const codePrefixLength = 8

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

// GetByCode looks the code up by its SHA-256 hash. The caller passes the
// plaintext value it received from the client; the plaintext is never used in a
// WHERE clause, so a database dump or a slow-query log cannot leak a usable
// authorization code (手册 9.5).
func (r Repository) GetByCode(ctx context.Context, code string) (*domain.AuthorizationCode, error) {
	var row authorizationCodeRow
	err := r.db.QueryRow(ctx, queryGetAuthorizationCodeByCode, platformcrypto.HashTokenValue(code)).Scan(
		&row.ID,
		&row.UserID,
		&row.Username,
		&row.OrganizationID,
		&row.OrganizationSlug,
		&row.ApplicationID,
		&row.ApplicationSlug,
		&row.RedirectURI,
		&row.Scopes,
		&row.State,
		&row.Nonce,
		&row.CodeChallenge,
		&row.ChallengeMethod,
		&row.Resource,
		&row.Sid,
		&row.AuthTime,
		&row.ExpiresAt,
		&row.UsedAt,
		&row.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("authorization code not found")
		}
		return nil, err
	}
	return toAuthorizationCode(row), nil
}

func (r Repository) Create(ctx context.Context, authorizationCode *domain.AuthorizationCode) error {
	// scopes is NOT NULL with a '{}' default; pgx encodes a nil slice as NULL and
	// bypasses the default, so a request without scope would fail the insert.
	scopes := authorizationCode.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	_, err := r.db.Exec(ctx, queryInsertAuthorizationCode,
		authorizationCode.ID,
		platformcrypto.HashTokenValue(authorizationCode.Code),
		codePrefix(authorizationCode.Code),
		authorizationCode.UserID,
		authorizationCode.ApplicationID,
		authorizationCode.RedirectURI,
		scopes,
		authorizationCode.State,
		authorizationCode.Sid,
		authorizationCode.Nonce,
		authorizationCode.Resource,
		authorizationCode.CodeChallenge,
		authorizationCode.ChallengeMethod,
		authorizationCode.AuthTime,
		authorizationCode.ExpiresAt,
		authorizationCode.CreatedAt,
	)
	return err
}

// MarkUsed consumes the code exactly once. Zero affected rows means either the
// code was already redeemed (concurrent redemption) or the row is gone; both are
// reported as a conflict so the caller cannot mistake a replay for success.
func (r Repository) MarkUsed(ctx context.Context, id string, usedAt time.Time) error {
	result, err := r.db.Exec(ctx, queryMarkAuthorizationCodeUsed, id, usedAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewConflict("authorization code already used")
	}
	return nil
}

func codePrefix(code string) string {
	if len(code) <= codePrefixLength {
		return code
	}
	return code[:codePrefixLength]
}

var _ domain.Repository = Repository{}
