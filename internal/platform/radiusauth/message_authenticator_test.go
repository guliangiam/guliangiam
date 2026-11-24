package radiusauth

import (
	"testing"

	"github.com/stretchr/testify/require"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2869"
)

func TestSignAndVerifyResponse(t *testing.T) {
	request := radius.New(radius.CodeAccessRequest, []byte("shared-secret"))
	require.NoError(t, rfc2865.UserName_SetString(request, "alice"))
	require.NoError(t, Sign(request))

	response := request.Response(radius.CodeAccessAccept)
	require.NoError(t, Sign(response))
	require.NoError(t, VerifyResponse(response, request))
}

func TestVerifyResponseRejectsMissingAuthenticator(t *testing.T) {
	request := radius.New(radius.CodeAccessRequest, []byte("shared-secret"))
	require.NoError(t, Sign(request))
	response := request.Response(radius.CodeAccessAccept)

	err := VerifyResponse(response, request)
	require.ErrorIs(t, err, ErrMissing)
}

func TestVerifyResponseRejectsTamperedAuthenticator(t *testing.T) {
	request := radius.New(radius.CodeAccessRequest, []byte("shared-secret"))
	require.NoError(t, Sign(request))
	response := request.Response(radius.CodeAccessAccept)
	require.NoError(t, Sign(response))

	value := response.Get(rfc2869.MessageAuthenticator_Type)
	require.NotEmpty(t, value)
	value[0] ^= 0xff
	require.NoError(t, rfc2869.MessageAuthenticator_Set(response, value))

	require.ErrorIs(t, VerifyResponse(response, request), ErrInvalid)
}

func TestVerifyResponseRejectsIdentifierMismatch(t *testing.T) {
	request := radius.New(radius.CodeAccessRequest, []byte("shared-secret"))
	require.NoError(t, Sign(request))
	response := request.Response(radius.CodeAccessAccept)
	response.Identifier++
	require.NoError(t, Sign(response))

	err := VerifyResponse(response, request)
	require.ErrorContains(t, err, "identifier")
}
