package idv

import (
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testUserID(value string) *string { return &value }

func TestVerificationNotification_ApprovedTransition(t *testing.T) {
	verification := &domain.Verification{
		ID:             "verification-1",
		OrganizationID: "org-1",
		UserID:         testUserID("user-1"),
		Status:         domain.StatusApproved,
	}

	command, ok := verificationNotification(verification, domain.StatusInProgress)
	require.True(t, ok)
	assert.Equal(t, "org-1", command.OrganizationID)
	assert.Equal(t, "user-1", command.UserID)
	assert.Equal(t, "security", command.Category)
	assert.Equal(t, "idv.verification.completed", command.EventType)
	assert.Equal(t, "identity_verification", command.SourceType)
	assert.Equal(t, "verification-1", command.SourceID)
	assert.Contains(t, command.Title, "通过")
}

func TestVerificationNotification_RejectedTransition(t *testing.T) {
	reason := "证件号码不匹配"
	verification := &domain.Verification{
		ID:             "verification-2",
		OrganizationID: "org-1",
		UserID:         testUserID("user-1"),
		Status:         domain.StatusRejected,
		Reason:         &reason,
	}

	command, ok := verificationNotification(verification, domain.StatusInProgress)
	require.True(t, ok)
	assert.Contains(t, command.Title, "未通过")
	// 拒绝原因来自 provider，可能带证件信息，不能写入收件箱。
	assert.NotContains(t, command.Content, reason)
}

func TestVerificationNotification_SkipsAnonymousSubject(t *testing.T) {
	verification := &domain.Verification{
		ID:             "verification-3",
		OrganizationID: "org-1",
		Status:         domain.StatusApproved,
	}

	_, ok := verificationNotification(verification, domain.StatusInProgress)
	assert.False(t, ok)
}

func TestVerificationNotification_SkipsNonTerminalState(t *testing.T) {
	verification := &domain.Verification{
		ID:             "verification-4",
		OrganizationID: "org-1",
		UserID:         testUserID("user-1"),
		Status:         domain.StatusPending,
	}

	_, ok := verificationNotification(verification, domain.StatusInProgress)
	assert.False(t, ok)
}

func TestVerificationNotification_SkipsDuplicateObservation(t *testing.T) {
	verification := &domain.Verification{
		ID:             "verification-5",
		OrganizationID: "org-1",
		UserID:         testUserID("user-1"),
		Status:         domain.StatusApproved,
	}

	_, ok := verificationNotification(verification, domain.StatusApproved)
	assert.False(t, ok)
}
