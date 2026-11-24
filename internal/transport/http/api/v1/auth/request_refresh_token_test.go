package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeExchangeAuthorizationCodeRequest_RefreshTokenGrant(t *testing.T) {
	form := url.Values{
		"grant_type":            {"refresh_token"},
		"refresh_token":         {"gl_prt_old"},
		"client_id":             {"refresh-client"},
		"client_assertion":      {"header.payload.signature"},
		"client_assertion_type": {"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens/exchange", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	decoded, err := decodeExchangeAuthorizationCodeRequest(req)
	require.NoError(t, err)
	require.True(t, decoded.isRefreshTokenGrant())
	require.False(t, decoded.isClientCredentialsGrant())
	require.Equal(t, "gl_prt_old", decoded.RefreshToken)
	require.Equal(t, "refresh-client", decoded.ClientID)
	require.Equal(t, "private_key_jwt", decoded.ClientAuthMethod)

	command := decoded.toRefreshTokenCommand(req)
	require.Equal(t, "gl_prt_old", command.RefreshToken)
	require.Equal(t, "private_key_jwt", command.ClientAuthMethod)
	require.NotEmpty(t, command.TokenEndpointURL)
}
