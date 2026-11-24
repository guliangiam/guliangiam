package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

// TestPasswordLogin_LockedAccountSkipsPasswordHashVerification proves the
// AC-AUTH-006 ordering clause: the locked path must answer with the generic
// credential rejection and leave the attempt counter untouched instead of
// reaching password-hash verification (this user's algorithm has no verifier).
func TestPasswordLogin_LockedAccountSkipsPasswordHashVerification(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	lockedUntil := now.Add(10 * time.Minute)

	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) {
		u.Credentials.PasswordAlgo = "g38a-no-such-credential-manager"
		u.Credentials.PasswordHash = "g38a-unreachable-hash"
	})
	uow := newTestUow(org, user, newTestApp())
	uow.lockoutAttempt.attempts = map[string]*lockoutattempts.Attempt{
		testUserID: {
			ID:          "lockout-g38a",
			UserID:      testUserID,
			Attempts:    5,
			LockedUntil: &lockedUntil,
			CreatedAt:   now.Add(-20 * time.Minute),
			UpdatedAt:   now.Add(-20 * time.Minute),
		},
	}
	records := &auditPlainRecords{}
	outbox := &auditPlainOutbox{}
	auditUOW := &loginFailureAuditUOW{
		loginTestUnitOfWork: uow,
		records:             records,
		outbox:              outbox,
	}
	svc := newLoginFailureAuditService(auditUOW)

	_, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         "g38a-any-password",
		ApplicationID:    testAppID,
		IPAddress:        "203.0.113.11",
		UserAgent:        "go-test-agent",
	})
	require.Error(t, err)

	typed, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeUnauthorized, typed.Code,
		"a locked account must return the generic credential error before any hash verification")
	assert.Equal(t, "invalid username or password", typed.Message)

	attempt, getErr := uow.lockoutAttempt.GetByUserID(context.Background(), testUserID)
	require.NoError(t, getErr)
	require.NotNil(t, attempt)
	assert.Equal(t, 5, attempt.Attempts, "the short-circuit must not increment the counter")
	assert.Equal(t, lockedUntil, *attempt.LockedUntil)

	auditRecords := failureAuditRecords(records.created)
	require.Len(t, auditRecords, 1, "exactly one lockout rejection audit record")
	assert.Equal(t, audit.EventAuthLoginFailed, auditRecords[0].Action)
	metadata := failureAuditMetadata(t, auditRecords[0])
	assert.Equal(t, "account_locked", metadata["reason"])
	assert.NotContains(t, typed.Message, "locked")
}
