package auth

import (
	"context"
	"testing"
	"time"

	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func challengeRejectionService(t *testing.T, now time.Time, challenge *mfachallengedomain.Challenge) (*Service, *loginTestChallengeRepository) {
	t.Helper()
	repo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{}}
	if challenge != nil {
		repo.items[challenge.ID] = challenge
	}
	return &Service{Clock: fakeClock{fixedTime: now}, UOW: loginTestUnitOfWork{challengeRepo: repo}}, repo
}

func newPendingChallenge(now time.Time) *mfachallengedomain.Challenge {
	return &mfachallengedomain.Challenge{
		ID:             "challenge-reject",
		OrganizationID: "org-1",
		UserID:         "user-1",
		State:          mfachallengedomain.StatePending,
		MethodType:     mfaTypeTotp,
		ExpiresAt:      now.Add(5 * time.Minute),
		CreatedAt:      now.Add(-time.Minute),
		UpdatedAt:      now,
		Revision:       1,
	}
}

func TestLoginChallengeRejectsMissingSessionPrincipal(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	svc, _ := challengeRejectionService(t, now, newPendingChallenge(now))

	_, err := svc.ApproveLoginChallenge(context.Background(), LoginChallengeCommand{ChallengeID: "challenge-reject"})
	if err == nil || apperrors.FromError(err).Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized without a session principal, got %v", err)
	}
}

func TestLoginChallengeRejectsExpiredChallengeAndDeletesIt(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	expired := newPendingChallenge(now)
	expired.ExpiresAt = now.Add(-time.Second)
	svc, repo := challengeRejectionService(t, now, expired)

	_, err := svc.ApproveLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID:           expired.ID,
		SessionUserID:         expired.UserID,
		SessionOrganizationID: expired.OrganizationID,
	})
	if err == nil || apperrors.FromError(err).Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized for expired challenge, got %v", err)
	}
	if _, ok := repo.items[expired.ID]; ok {
		t.Fatalf("expected expired challenge to be deleted")
	}
}

func TestLoginChallengeRejectsReplayedConsumedChallenge(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	svc, repo := challengeRejectionService(t, now, newPendingChallenge(now))

	// Simulate a challenge already consumed by a previous successful MFA step.
	delete(repo.items, "challenge-reject")

	_, err := svc.GetLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID:           "challenge-reject",
		SessionUserID:         "user-1",
		SessionOrganizationID: "org-1",
	})
	if err == nil || apperrors.FromError(err).Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized for consumed challenge, got %v", err)
	}
}

func TestLoginChallengeRejectsCrossTenantAccess(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	svc, _ := challengeRejectionService(t, now, newPendingChallenge(now))

	_, err := svc.GetLoginChallenge(context.Background(), LoginChallengeCommand{
		ChallengeID:           "challenge-reject",
		SessionUserID:         "user-1",
		SessionOrganizationID: "org-other",
	})
	if err == nil || apperrors.FromError(err).Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized for cross-tenant challenge access, got %v", err)
	}
}
