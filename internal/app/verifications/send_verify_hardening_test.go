package verifications

import (
	"context"
	"testing"
	"time"

	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPepper() ([]byte, error) { return []byte("unit-test-pepper"), nil }

// ---------------------------------------------------------------------------
// BE-A-04: a new code supersedes still-pending codes for the same recipient
// ---------------------------------------------------------------------------

func TestSendVerification_SupersedesOldPending(t *testing.T) {
	old := &verificationsdomain.Verification{
		ID:             "old-verification",
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "victim@example.com",
		Code:           "111111",
		Status:         "pending",
		ExpiresAt:      fakeClock{}.Now().Add(5 * time.Minute),
	}
	repo := &fakeVerificationRepo{active: old}
	svc := Service{
		Clock:  fakeClock{},
		UUID:   fakeUUID{id: "new-verification"},
		UOW:    fakeAuthUoW{repo: repo},
		pepper: testPepper,
	}

	result, err := svc.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "victim@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "superseded", old.Status, "old pending code must be invalidated by the new send")
	require.Len(t, repo.created, 1)
	assert.Equal(t, "pending", repo.created[0].Status)
	assert.Equal(t, result.ID, repo.created[0].ID)
}

func TestSendVerification_DeliveryFailureKeepsOldPending(t *testing.T) {
	old := &verificationsdomain.Verification{
		ID:             "old-verification",
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "victim@example.com",
		Code:           "111111",
		Status:         "pending",
		ExpiresAt:      fakeClock{}.Now().Add(5 * time.Minute),
	}
	repo := &fakeVerificationRepo{active: old}
	sender := &captureEmailSender{err: assert.AnError}
	svc := Service{
		Clock:          fakeClock{},
		UUID:           fakeUUID{id: "new-verification"},
		UOW:            fakeAuthUoW{repo: repo},
		SenderResolver: &fakeSenderResolver{email: sender},
		pepper:         testPepper,
	}

	_, err := svc.SendVerification(context.Background(), SendVerificationCommand{
		OrganizationID: testOrganizationID,
		Type:           VerificationTypeEmail,
		Recipient:      "victim@example.com",
	})
	require.Error(t, err)
	assert.Equal(t, "pending", old.Status, "a failed delivery must leave the previous code usable")
	assert.Empty(t, repo.created)
}

// ---------------------------------------------------------------------------
// BE-A-16: numeric codes are unbiased and well-formed
// ---------------------------------------------------------------------------

func TestGenerateNumericCode_FormatAndDigitCoverage(t *testing.T) {
	seen := make(map[byte]bool)
	for i := 0; i < 2000; i++ {
		code, err := generateNumericCode(verificationCodeLength)
		require.NoError(t, err)
		require.Len(t, code, verificationCodeLength)
		for j := 0; j < len(code); j++ {
			require.GreaterOrEqual(t, code[j], byte('0'))
			require.LessOrEqual(t, code[j], byte('9'))
			seen[code[j]] = true
		}
	}
	assert.Len(t, seen, 10, "every digit 0-9 must be reachable")
}

// ---------------------------------------------------------------------------
// BE-A-20: VerifyCode honours the issuing organization when supplied
// ---------------------------------------------------------------------------

func legacyPendingVerification(orgID string) *verificationsdomain.Verification {
	return &verificationsdomain.Verification{
		ID:             "verification-1",
		OrganizationID: orgID,
		Type:           VerificationTypeEmail,
		Recipient:      "user@example.com",
		Code:           "123456", // legacy plaintext row, matched without digest
		Status:         "pending",
		ExpiresAt:      fakeClock{}.Now().Add(5 * time.Minute),
	}
}

func TestVerifyCode_OrgMismatchRejected(t *testing.T) {
	active := legacyPendingVerification(testOrganizationID)
	repo := &fakeVerificationRepo{active: active}
	svc := Service{Clock: fakeClock{}, UOW: fakeAuthUoW{repo: repo}, pepper: testPepper}

	err := svc.VerifyCode(context.Background(), VerifyCodeCommand{
		Type:           VerificationTypeEmail,
		Recipient:      "user@example.com",
		Code:           "123456",
		OrganizationID: "00000000-0000-0000-0000-000000000002",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Equal(t, "pending", active.Status, "cross-tenant code must not be consumed")
}

func TestVerifyCode_OrgMatchSucceeds(t *testing.T) {
	active := legacyPendingVerification(testOrganizationID)
	repo := &fakeVerificationRepo{active: active}
	svc := Service{Clock: fakeClock{}, UOW: fakeAuthUoW{repo: repo}, pepper: testPepper}

	err := svc.VerifyCode(context.Background(), VerifyCodeCommand{
		Type:           VerificationTypeEmail,
		Recipient:      "user@example.com",
		Code:           "123456",
		OrganizationID: testOrganizationID,
	})
	require.NoError(t, err)
	assert.Equal(t, "verified", active.Status)
}

func TestVerifyCode_EmptyOrgKeepsLegacyBehaviour(t *testing.T) {
	active := legacyPendingVerification(testOrganizationID)
	repo := &fakeVerificationRepo{active: active}
	svc := Service{Clock: fakeClock{}, UOW: fakeAuthUoW{repo: repo}, pepper: testPepper}

	// The public FE verify modal sends no organizationId; empty must keep
	// verifying so the existing flow does not break.
	err := svc.VerifyCode(context.Background(), VerifyCodeCommand{
		Type:      VerificationTypeEmail,
		Recipient: "user@example.com",
		Code:      "123456",
	})
	require.NoError(t, err)
	assert.Equal(t, "verified", active.Status)
}
