package auth

import (
	"context"
	"testing"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
)

type fakeRefreshExchanger struct {
	requests []RefreshTokenExchangeRequest
	result   RefreshTokenExchangeResult
	err      error
}

func (f *fakeRefreshExchanger) ExchangeRefreshToken(_ context.Context, request RefreshTokenExchangeRequest) (RefreshTokenExchangeResult, error) {
	f.requests = append(f.requests, request)
	return f.result, f.err
}

func refreshGrantFixture(t *testing.T, grantTypes []string, exchanger *fakeRefreshExchanger) *Service {
	t.Helper()
	app := &applicationdomain.Application{
		ID:                      testAppID,
		OrganizationID:          testOrgID,
		ClientID:                "refresh-client",
		TokenEndpointAuthMethod: applicationdomain.TokenEndpointAuthMethodNone,
		GrantTypes:              grantTypes,
	}
	repo := &loginTestAppRepository{
		apps:       map[string]*applicationdomain.Application{app.ID: app},
		byClientID: map[string]*applicationdomain.Application{app.ClientID: app},
	}
	base := newServiceTestUow(newTestOrg(), newTestUser(), app)
	base.appRepo = repo
	svc := newTestService(base.loginTestUnitOfWork)
	svc.UOW = base
	svc.RefreshTokens = exchanger
	return &svc
}

func TestExchangeRefreshToken_IssuesRotatedPair(t *testing.T) {
	exchanger := &fakeRefreshExchanger{result: RefreshTokenExchangeResult{
		AccessToken: "gl_pat_new", RefreshToken: "gl_prt_new", ExpiresAt: time.Now().Add(time.Hour),
		Scopes: []string{"openid", "profile"}, TokenType: "Bearer",
	}}
	svc := refreshGrantFixture(t, []string{grantAuthorizationCode, oauthRefreshToken}, exchanger)

	response, err := svc.ExchangeRefreshToken(context.Background(), ExchangeRefreshTokenCommand{
		RefreshToken: "gl_prt_old",
		ClientID:     "refresh-client",
	})
	require.NoError(t, err)
	require.Equal(t, "gl_pat_new", response.AccessToken)
	require.Equal(t, "gl_prt_new", response.RefreshToken)
	require.Equal(t, "openid profile", response.Scope)
	require.Len(t, exchanger.requests, 1)
	require.Equal(t, testAppID, exchanger.requests[0].ApplicationID)
	require.Equal(t, testOrgID, exchanger.requests[0].OrganizationID)
}

func TestExchangeRefreshToken_RequiresRegisteredGrant(t *testing.T) {
	svc := refreshGrantFixture(t, []string{grantAuthorizationCode}, &fakeRefreshExchanger{})
	_, err := svc.ExchangeRefreshToken(context.Background(), ExchangeRefreshTokenCommand{
		RefreshToken: "gl_prt_old",
		ClientID:     "refresh-client",
	})
	require.Error(t, err)
	var protocolErr apperrors.ProtocolCodeError
	require.ErrorAs(t, err, &protocolErr)
	require.Equal(t, "invalid_grant", protocolErr.ProtocolCode())
}

func TestExchangeRefreshToken_MapsRotationErrorsToInvalidGrant(t *testing.T) {
	svc := refreshGrantFixture(t, []string{grantAuthorizationCode, oauthRefreshToken}, &fakeRefreshExchanger{
		err: apperrors.NewInvalidArgument("refresh token is invalid or expired"),
	})
	_, err := svc.ExchangeRefreshToken(context.Background(), ExchangeRefreshTokenCommand{
		RefreshToken: "gl_prt_old",
		ClientID:     "refresh-client",
	})
	require.Error(t, err)
	var protocolErr apperrors.ProtocolCodeError
	require.ErrorAs(t, err, &protocolErr)
	require.Equal(t, "invalid_grant", protocolErr.ProtocolCode())
}

func TestExchangeRefreshToken_RequiresRefreshToken(t *testing.T) {
	svc := refreshGrantFixture(t, []string{grantAuthorizationCode, oauthRefreshToken}, &fakeRefreshExchanger{})
	_, err := svc.ExchangeRefreshToken(context.Background(), ExchangeRefreshTokenCommand{
		ClientID: "refresh-client",
	})
	require.Error(t, err)
}
