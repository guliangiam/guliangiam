package auth

import (
	"context"
	"log/slog"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// tokenFamily tracks a family of tokens derived from the same authorization.
// When token reuse is detected (a previously-rotated refresh token is presented again),
// the entire family should be revoked to prevent token theft.
type tokenFamily struct {
	FamilyID       string
	AccessTokenID  string
	RefreshTokenID string
}

// detectTokenReuse checks if a refresh token has already been used.
// If the token is already revoked, it was previously used, indicating potential token theft.
func detectTokenReuse(refreshTokenID string, isRevoked bool) bool {
	return isRevoked
}

// revokeTokenFamilyUoW revokes all tokens derived from the same authorization.
// This is the recommended response to token reuse detection per OAuth 2.1.
func revokeTokenFamilyUoW(ctx context.Context, uow appuow.UnitOfWork, familyID string) error {
	if familyID == "" {
		return nil
	}

	now := time.Now()
	affected, err := uow.Tokens().RevokeByFamilyID(ctx, familyID, now)
	if err != nil {
		return apperrors.WrapInternal(err, "revoke token family")
	}

	slog.InfoContext(ctx, "token family revoked due to reuse detection",
		"family_id", familyID,
		"revoked_count", affected,
	)

	return nil
}

// revokeTokenFamily is the function variable for token family revocation.
// It is wired during bootstrap via WireTokenFamilyRevocation.
var revokeTokenFamily = func(familyID string) error {
	// Default no-op if not wired.
	return nil
}

// WireTokenFamilyRevocation connects the token family revocation function
// to the unit of work-based implementation. Call this once during bootstrap.
func WireTokenFamilyRevocation(getUoW func() appuow.UnitOfWork) {
	revokeTokenFamily = func(familyID string) error {
		ctx := context.Background()
		uow := getUoW()
		return revokeTokenFamilyUoW(ctx, uow, familyID)
	}
}

// CheckTokenReuseAndRevoke checks for token reuse and revokes the entire family
// if reuse is detected. Returns an error describing the reuse detection.
func CheckTokenReuseAndRevoke(familyID string, isRevoked bool) error {
	if !detectTokenReuse("", isRevoked) {
		return nil
	}

	if err := revokeTokenFamily(familyID); err != nil {
		return apperrors.NewInternal("failed to revoke token family after reuse detection: " + err.Error())
	}

	return apperrors.NewUnauthorized("token reuse detected: all tokens in the family have been revoked")
}
