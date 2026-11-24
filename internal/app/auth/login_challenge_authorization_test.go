package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	otherUserID = "019usr-0000-0000-0000-0000000000ff"
	otherOrgID  = "019org-0000-0000-0000-0000000000ff"
)

func newLoginChallengeTestService(t *testing.T, challenges ...*mfachallengedomain.Challenge) (Service, *loginTestChallengeRepository) {
	t.Helper()

	uow := newTestUow(newTestOrg(), newTestUser(), newTestApp())
	repo := uow.challengeRepo
	repo.items = make(map[string]*mfachallengedomain.Challenge, len(challenges))
	for _, challenge := range challenges {
		repo.items[challenge.ID] = challenge
	}
	return newTestService(uow), repo
}

func pendingLoginChallenge(id, orgID, userID string) *mfachallengedomain.Challenge {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	return &mfachallengedomain.Challenge{
		ID:             id,
		OrganizationID: orgID,
		UserID:         userID,
		ApplicationID:  testAppID,
		State:          mfachallengedomain.StatePending,
		MethodType:     loginMethodMFA,
		ExpiresAt:      now.Add(time.Hour),
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
}

func assertLoginChallengeRejected(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	typed, ok := err.(*apperrors.Error)
	require.True(t, ok, "error type = %T, want *apperrors.Error", err)
	assert.Equal(t, apperrors.CodeUnauthorized, typed.Code)
	assert.Equal(t, "mfa challenge is invalid or expired", typed.Message)
	assert.Empty(t, typed.Meta, "rejection must not carry challenge details")
}

func TestApproveLoginChallenge_OwnerSucceeds(t *testing.T) {
	challenge := pendingLoginChallenge("challenge-own", testOrgID, testUserID)
	svc, repo := newLoginChallengeTestService(t, challenge)

	response, err := svc.ApproveLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID:           challenge.ID,
		SessionUserID:         testUserID,
		SessionOrganizationID: testOrgID,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, mfachallengedomain.StateApproved, response.State)
	assert.Equal(t, mfachallengedomain.StateApproved, repo.items[challenge.ID].State)
	assert.NotNil(t, repo.items[challenge.ID].VerifiedAt)
}

func TestRejectLoginChallenge_OwnerSucceeds(t *testing.T) {
	challenge := pendingLoginChallenge("challenge-reject-own", testOrgID, testUserID)
	svc, repo := newLoginChallengeTestService(t, challenge)

	response, err := svc.RejectLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID:           challenge.ID,
		SessionUserID:         testUserID,
		SessionOrganizationID: testOrgID,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, mfachallengedomain.StateRejected, response.State)
	assert.Equal(t, mfachallengedomain.StateRejected, repo.items[challenge.ID].State)
}

func TestApproveLoginChallenge_OtherTenantUserRejected(t *testing.T) {
	challenge := pendingLoginChallenge("challenge-victim", testOrgID, testUserID)
	svc, repo := newLoginChallengeTestService(t, challenge)

	_, err := svc.ApproveLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID:           challenge.ID,
		SessionUserID:         otherUserID,
		SessionOrganizationID: testOrgID,
	})
	assertLoginChallengeRejected(t, err)
	assert.Equal(t, mfachallengedomain.StatePending, repo.items[challenge.ID].State,
		"a foreign session must not mutate the challenge")
}

func TestApproveLoginChallenge_CrossTenantRejected(t *testing.T) {
	challenge := pendingLoginChallenge("challenge-cross-tenant", otherOrgID, otherUserID)
	svc, repo := newLoginChallengeTestService(t, challenge)

	_, err := svc.ApproveLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID:           challenge.ID,
		SessionUserID:         testUserID,
		SessionOrganizationID: testOrgID,
	})
	assertLoginChallengeRejected(t, err)
	assert.Equal(t, mfachallengedomain.StatePending, repo.items[challenge.ID].State)
}

func TestApproveLoginChallenge_MissingSessionRejected(t *testing.T) {
	challenge := pendingLoginChallenge("challenge-no-session", testOrgID, testUserID)
	svc, repo := newLoginChallengeTestService(t, challenge)

	_, err := svc.ApproveLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID: challenge.ID,
	})
	assertLoginChallengeRejected(t, err)
	assert.Equal(t, mfachallengedomain.StatePending, repo.items[challenge.ID].State,
		"no session must never be treated as an implicit approval")
}

func TestApproveLoginChallenge_MissingAndForeignChallengesAreIndistinguishable(t *testing.T) {
	challenge := pendingLoginChallenge("challenge-existing", testOrgID, testUserID)
	svc, _ := newLoginChallengeTestService(t, challenge)

	_, missingErr := svc.ApproveLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID:           "challenge-does-not-exist",
		SessionUserID:         testUserID,
		SessionOrganizationID: testOrgID,
	})
	_, foreignErr := svc.ApproveLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID:           challenge.ID,
		SessionUserID:         otherUserID,
		SessionOrganizationID: testOrgID,
	})

	assertLoginChallengeRejected(t, missingErr)
	assertLoginChallengeRejected(t, foreignErr)

	missingTyped := missingErr.(*apperrors.Error)
	foreignTyped := foreignErr.(*apperrors.Error)
	assert.Equal(t, missingTyped.Code, foreignTyped.Code)
	assert.Equal(t, missingTyped.Message, foreignTyped.Message)
	assert.Equal(t, missingTyped.HTTPStatus(), foreignTyped.HTTPStatus())
}

func TestGetLoginChallenge_RequiresOwnerSession(t *testing.T) {
	challenge := pendingLoginChallenge("challenge-read", testOrgID, testUserID)
	svc, _ := newLoginChallengeTestService(t, challenge)

	response, err := svc.GetLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID:           challenge.ID,
		SessionUserID:         testUserID,
		SessionOrganizationID: testOrgID,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, challenge.ID, response.ID)
	assert.Equal(t, mfachallengedomain.StatePending, response.State)

	for _, tc := range []struct {
		name   string
		userID string
		orgID  string
	}{
		{name: "no session"},
		{name: "foreign user", userID: otherUserID, orgID: testOrgID},
		{name: "cross tenant", userID: otherUserID, orgID: otherOrgID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GetLoginChallenge(context.Background(), LoginChallengeCommand{
				ChallengeID:           challenge.ID,
				SessionUserID:         tc.userID,
				SessionOrganizationID: tc.orgID,
			})
			assertLoginChallengeRejected(t, err)
		})
	}
}
