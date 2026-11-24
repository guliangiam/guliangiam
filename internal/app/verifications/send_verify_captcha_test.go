package verifications

import (
	"context"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendVerification_CaptchaRejection_BlocksDelivery(t *testing.T) {
	sender := &captureEmailSender{}
	service, repo := newTestService(&fakeSenderResolver{email: sender})
	service.CaptchaVerifier = func(context.Context, string, string, string, string) error {
		return apperrors.NewInvalidArgument("captcha verification failed")
	}

	_, err := service.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "user@example.com",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Zero(t, sender.calls)
	assert.Empty(t, repo.created)
}

func TestSendVerification_CaptchaApproved_AllowsDelivery(t *testing.T) {
	sender := &captureEmailSender{}
	service, repo := newTestService(&fakeSenderResolver{email: sender})
	var gotOrg, gotToken, gotID, gotIP string
	service.CaptchaVerifier = func(_ context.Context, organizationID, captchaToken, captchaID, remoteIP string) error {
		gotOrg, gotToken, gotID, gotIP = organizationID, captchaToken, captchaID, remoteIP
		return nil
	}

	_, err := service.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "user@example.com",
		CaptchaToken:   "tok",
		CaptchaID:      "cap-1",
		ClientIP:       "1.2.3.4",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, sender.calls)
	assert.Len(t, repo.created, 1)
	assert.Equal(t, testOrganizationID, gotOrg)
	assert.Equal(t, "tok", gotToken)
	assert.Equal(t, "cap-1", gotID)
	assert.Equal(t, "1.2.3.4", gotIP)
}

func TestSendVerification_NilCaptchaVerifier_SkipsGate(t *testing.T) {
	sender := &captureEmailSender{}
	service, _ := newTestService(&fakeSenderResolver{email: sender})
	require.Nil(t, service.CaptchaVerifier)

	_, err := service.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "user@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, sender.calls)
}
