package tokens

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func toToken(row tokenRow) (*domain.Token, error) {
	token := &domain.Token{
		ID:                    row.ID,
		UserID:                row.UserID,
		Username:              row.Username,
		OrganizationID:        row.OrganizationID,
		OrganizationSlug:      row.OrganizationSlug,
		ApplicationID:         row.ApplicationID,
		ApplicationSlug:       row.ApplicationSlug,
		TokenPrefix:           row.TokenPrefix,
		TokenHash:             row.TokenHash,
		DeviceSecretHash:      row.DeviceSecretHash,
		DeviceSecretExpiresIn: row.DeviceSecretExpiresIn,
		ExpiresAt:             row.ExpiresAt,
		RevokedAt:             row.RevokedAt,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
		Revision:              row.Revision,
	}

	if err := shared.UnmarshalJSON(row.Metadata, &token.Metadata); err != nil {
		return nil, apperrors.NewInternal("decode token metadata")
	}

	token.Scopes = row.Scopes
	token.Audience = row.Audience

	if token.Metadata == nil {
		token.Metadata = map[string]any{}
	}
	if token.Scopes == nil {
		token.Scopes = []string{}
	}
	if token.Audience == nil {
		token.Audience = []string{}
	}

	return token, nil
}

func toRefreshToken(row refreshTokenRow) *domain.RefreshToken {
	refreshToken := &domain.RefreshToken{
		ID:               row.ID,
		AccessTokenID:    row.AccessTokenID,
		UserID:           row.UserID,
		Username:         row.Username,
		OrganizationID:   row.OrganizationID,
		OrganizationSlug: row.OrganizationSlug,
		ApplicationID:    row.ApplicationID,
		ApplicationSlug:  row.ApplicationSlug,
		RefreshTokenHash: row.RefreshTokenHash,
		FamilyID:         row.FamilyID,
		ExpiresAt:        row.ExpiresAt,
		RevokedAt:        row.RevokedAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Revision:         row.Revision,
	}

	refreshToken.Scopes = row.Scopes
	refreshToken.Audience = row.Audience

	if refreshToken.Scopes == nil {
		refreshToken.Scopes = []string{}
	}
	if refreshToken.Audience == nil {
		refreshToken.Audience = []string{}
	}

	return refreshToken
}
