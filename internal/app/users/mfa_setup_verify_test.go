package users

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestTotpMfaUtil_SetupVerify_InvalidPasscode(t *testing.T) {
	util := newTotpMfaUtil(&mfaSetupProps{MfaType: "app", Secret: "JBSWY3DPEHPK3PXP"})
	err := util.SetupVerify("000000") // Very likely invalid passcode
	assert.Error(t, err)
}

func TestSmsMfaUtil_SetupVerify(t *testing.T) {
	util := newSmsMfaUtil(&mfaSetupProps{MfaType: "sms", Secret: "test-secret", CountryCode: "+86"})
	err := util.SetupVerify("123456")
	// smsSetupVerify is defined elsewhere; this tests the delegation
	_ = err
}

func TestEmailMfaUtil_SetupVerify(t *testing.T) {
	util := newEmailMfaUtil(&mfaSetupProps{MfaType: "email", Secret: "test-secret"})
	err := util.SetupVerify("123456")
	// emailSetupVerify is defined elsewhere; this tests the delegation
	_ = err
}

func TestRadiusMfaUtil_SetupVerify_NilProps(t *testing.T) {
	util := newRadiusMfaUtil(&mfaSetupProps{MfaType: "radius"})
	err := util.SetupVerify("123456")
	// Will fail because Secret is empty
	assert.Error(t, err)
}

func TestRadiusMfaUtil_SetupVerify_EmptySecret(t *testing.T) {
	util := newRadiusMfaUtil(&mfaSetupProps{MfaType: "radius", Secret: ""})
	err := util.SetupVerify("123456")
	assert.Error(t, err)
	// Should be InvalidArgument
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
}

func TestRadiusMfaUtil_SetupVerify_WithSecret(t *testing.T) {
	// Save original and restore after test
	origRadiusAuthenticate := radiusAuthenticate
	defer func() { radiusAuthenticate = origRadiusAuthenticate }()

	// Mock the radius authenticate function
	radiusAuthenticate = func(host, sharedSecret, username, passcode string) error {
		return nil // Simulate successful auth
	}

	util := newRadiusMfaUtil(&mfaSetupProps{MfaType: "radius", Secret: "radius-server", URL: "shared-secret", CountryCode: "testuser"})
	err := util.SetupVerify("123456")
	assert.NoError(t, err)
}

func TestRadiusMfaUtil_SetupVerify_RadiusError(t *testing.T) {
	origRadiusAuthenticate := radiusAuthenticate
	defer func() { radiusAuthenticate = origRadiusAuthenticate }()

	radiusAuthenticate = func(host, sharedSecret, username, passcode string) error {
		return apperrors.NewInternal("radius server connection failed")
	}

	util := newRadiusMfaUtil(&mfaSetupProps{MfaType: "radius", Secret: "radius-server", URL: "shared-secret", CountryCode: "testuser"})
	err := util.SetupVerify("123456")
	assert.Error(t, err)
}

func TestPushMfaUtil_SetupVerify_NotApproved(t *testing.T) {
	origPushSetupVerifyFn := pushSetupVerifyFn
	defer func() { pushSetupVerifyFn = origPushSetupVerifyFn }()

	pushSetupVerifyFn = func(challengeID string) (bool, error) {
		return false, nil // Not approved
	}

	util := newPushMfaUtil(&mfaSetupProps{MfaType: "push"})
	err := util.SetupVerify("challenge-123")
	assert.Error(t, err)
	var appErr *apperrors.Error
	assert.ErrorAs(t, err, &appErr)
}

func TestPushMfaUtil_SetupVerify_VerifyError(t *testing.T) {
	origPushSetupVerifyFn := pushSetupVerifyFn
	defer func() { pushSetupVerifyFn = origPushSetupVerifyFn }()

	pushSetupVerifyFn = func(challengeID string) (bool, error) {
		return false, apperrors.NewInternal("push service unavailable")
	}

	util := newPushMfaUtil(&mfaSetupProps{MfaType: "push"})
	err := util.SetupVerify("challenge-123")
	assert.Error(t, err)
}
