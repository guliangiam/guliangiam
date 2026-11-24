package users

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeFinishRequest(t *testing.T, body string) AdminRegistrationFinishRequest {
	t.Helper()
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	var decoded AdminRegistrationFinishRequest
	require.NoError(t, route.DecodeJSON(req, &decoded))
	return decoded
}

func TestAdminRegistrationFinishRequest_AcceptsDerivedMaterialBody(t *testing.T) {
	decoded := decodeFinishRequest(t, `{
		"challengeId":"challenge-id",
		"challenge":"challenge",
		"clientDataJSON":"eA",
		"attestationObject":"eA",
		"transports":["internal"],
		"name":"Office key"
	}`)
	assert.Empty(t, decoded.forbiddenRegistrationFields())
	assert.Empty(t, decoded.CredentialID)
	assert.Equal(t, "Office key", decoded.Name)
}

func TestAdminRegistrationFinishRequest_RejectsForbiddenFields(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"publicKey", `{"challengeId":"c","challenge":"x","clientDataJSON":"eA","attestationObject":"eA","publicKey":"cose"}`, []string{"publicKey"}},
		{"aaguid", `{"challengeId":"c","challenge":"x","clientDataJSON":"eA","attestationObject":"eA","aaguid":"0000"}`, []string{"aaguid"}},
		{"signCount", `{"challengeId":"c","challenge":"x","clientDataJSON":"eA","attestationObject":"eA","signCount":0}`, []string{"signCount"}},
		{"all", `{"challengeId":"c","challenge":"x","clientDataJSON":"eA","attestationObject":"eA","publicKey":"a","aaguid":"b","signCount":1}`, []string{"publicKey", "aaguid", "signCount"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decoded := decodeFinishRequest(t, tc.body)
			assert.Equal(t, tc.want, decoded.forbiddenRegistrationFields())
		})
	}
}

func TestFinishWebAuthnCredentialRegistration_ForbiddenFieldsRejectedBeforeService(t *testing.T) {
	handler := Handler{} // zero Service: a service call would panic
	_, err := handler.FinishWebAuthnCredentialRegistration(context.Background(), "org-1", "user-1", AdminRegistrationFinishRequest{
		ChallengeID:       "c",
		Challenge:         "x",
		ClientDataJSON:    "eA",
		AttestationObject: "eA",
		PublicKey:         []byte(`"cose"`),
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}
