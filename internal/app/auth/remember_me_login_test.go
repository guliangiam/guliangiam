package auth

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLoginCommandRememberMeTokenSubstitutesPasscode(t *testing.T) {
	base := LoginCommand{
		OrganizationSlug:  "acme",
		Username:          "alice",
		Password:          "secret",
		ApplicationID:     "app-1",
		EnableMfaRemember: true,
	}

	withToken := base
	withToken.RememberMeToken = "rm_token"
	require.NoError(t, validateLoginCommand(withToken))

	withoutToken := base
	err := validateLoginCommand(withoutToken)
	require.Error(t, err)
	assertErrorCode(t, err, apperrors.CodeInvalidArgument)

	// challengeId / mfaType rules are unchanged for token-less requests.
	challengeOnly := LoginCommand{
		OrganizationSlug: "acme",
		ChallengeID:      "challenge-1",
		ApplicationID:    "app-1",
	}
	require.Error(t, validateLoginCommand(challengeOnly))
	assertErrorCode(t, validateLoginCommand(challengeOnly), apperrors.CodeInvalidArgument)
}

func TestValidateLoginCommandRememberMeTokenWithChallengeAndMfaType(t *testing.T) {
	command := LoginCommand{
		OrganizationSlug: "acme",
		ChallengeID:      "challenge-1",
		MfaType:          "totp",
		RememberMeToken:  "rm_token",
	}
	require.NoError(t, validateLoginCommand(command))

	withoutToken := command
	withoutToken.RememberMeToken = ""
	err := validateLoginCommand(withoutToken)
	require.Error(t, err)
	assertErrorCode(t, err, apperrors.CodeInvalidArgument)
	assert.Contains(t, err.Error(), "passcode")
}
