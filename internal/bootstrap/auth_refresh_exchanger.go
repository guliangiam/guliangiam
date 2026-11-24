package bootstrap

import (
	"context"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apptokens "github.com/guliangiam/guliangiam/internal/app/tokens"
)

// refreshTokenExchanger adapts app/tokens to the auth refresh_token grant port.
// The composition root owns the cross-app wiring so app/auth imports no other
// app package.
type refreshTokenExchanger struct {
	tokens apptokens.Service
}

func newRefreshTokenExchanger(tokens apptokens.Service) *refreshTokenExchanger {
	return &refreshTokenExchanger{tokens: tokens}
}

func (a *refreshTokenExchanger) ExchangeRefreshToken(ctx context.Context, request appauth.RefreshTokenExchangeRequest) (appauth.RefreshTokenExchangeResult, error) {
	issued, err := a.tokens.RefreshAccessToken(ctx, apptokens.RefreshAccessTokenCommand{
		OrganizationID: request.OrganizationID,
		ApplicationID:  request.ApplicationID,
		RefreshToken:   request.RefreshToken,
	})
	if err != nil {
		return appauth.RefreshTokenExchangeResult{}, err
	}
	return appauth.RefreshTokenExchangeResult{
		AccessToken:  issued.TokenValue,
		RefreshToken: issued.RefreshToken,
		ExpiresAt:    issued.ExpiresAt,
		Scopes:       issued.Scopes,
		TokenType:    "Bearer",
	}, nil
}
