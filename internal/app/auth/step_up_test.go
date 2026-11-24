package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var stepUpTestNow = time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

// stepUpTestUnitOfWork reuses the login fakes but returns a session repository
// that can hold an active session and records which ids were updated.
type stepUpTestUnitOfWork struct {
	loginTestUnitOfWork
	sessions *stepUpTestSessionRepository
}

func (u stepUpTestUnitOfWork) Sessions() sessiondomain.Repository { return u.sessions }

func (u stepUpTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type stepUpTestSessionRepository struct {
	*loginTestSessionRepository
	updated []*sessiondomain.Session
}

func (r *stepUpTestSessionRepository) GetByIDForOrganization(_ context.Context, id, organizationID string) (*sessiondomain.Session, error) {
	session, ok := r.sessions[id]
	if !ok || session.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("session not found")
	}
	return session, nil
}

func (r *stepUpTestSessionRepository) Update(_ context.Context, session *sessiondomain.Session) error {
	cloned := *session
	r.updated = append(r.updated, &cloned)
	return nil
}

func newStepUpTestService(uow stepUpTestUnitOfWork) Service {
	return Service{
		Clock:                  fakeClock{fixedTime: stepUpTestNow},
		UUIDGenerator:          fakeUUIDGenerator{fixedID: "step-up-challenge-id"},
		UOW:                    uow,
		WebAuthnAllowedOrigins: []string{"https://login.example.com"},
	}
}

func TestAssuranceLevelForAuthenticationLevel(t *testing.T) {
	cases := map[string]string{
		"password": sessiondomain.AssuranceLevelAAL1,
		"":         sessiondomain.AssuranceLevelAAL1,
		"ldap":     sessiondomain.AssuranceLevelAAL1,
		"mfa":      sessiondomain.AssuranceLevelAAL2,
		"MFA":      sessiondomain.AssuranceLevelAAL2,
		"fido2":    sessiondomain.AssuranceLevelAAL2,
		"unknown":  sessiondomain.AssuranceLevelAAL1,
	}
	for level, want := range cases {
		assert.Equal(t, want, sessiondomain.AssuranceLevelForAuthenticationLevel(level), "level=%q", level)
	}
}

func TestSessionValidateAssuranceLevel(t *testing.T) {
	base := sessiondomain.Session{
		ID:             "sess-1",
		UserID:         "user-1",
		OrganizationID: "org-1",
		SessionState:   sessiondomain.StateActive,
		ExpiresAt:      stepUpTestNow.Add(time.Hour),
	}

	unset := base
	unset.AssuranceLevel = ""
	require.NoError(t, unset.Validate())

	aal2 := base
	aal2.AssuranceLevel = sessiondomain.AssuranceLevelAAL2
	require.NoError(t, aal2.Validate())

	bad := base
	bad.AssuranceLevel = "aal3"
	err := bad.Validate()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

func TestBeginStepUpRequiresBoundIdentifiers(t *testing.T) {
	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{}}
	uow := stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			userRepo:      &loginTestUserRepository{},
			challengeRepo: challengeRepo,
		},
	}
	service := newStepUpTestService(uow)

	cases := map[string]StepUpBeginCommand{
		"missing organization": {UserID: "user-1", SessionID: "sess-1", OperationScope: "sessions:read"},
		"missing user":         {OrganizationID: "org-1", SessionID: "sess-1", OperationScope: "sessions:read"},
		"missing session":      {OrganizationID: "org-1", UserID: "user-1", OperationScope: "sessions:read"},
		"missing scope":        {OrganizationID: "org-1", UserID: "user-1", SessionID: "sess-1", OperationScope: "  "},
		"scope too long":       {OrganizationID: "org-1", UserID: "user-1", SessionID: "sess-1", OperationScope: string(make([]rune, 129))},
	}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := service.BeginStepUp(context.Background(), command)
			require.Error(t, err)
			var appErr *apperrors.Error
			require.True(t, errors.As(err, &appErr))
			assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
		})
	}
	assert.Empty(t, challengeRepo.items)
}

func TestBeginStepUpCreatesBoundChallenge(t *testing.T) {
	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{}}
	uow := stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			userRepo:      &loginTestUserRepository{},
			challengeRepo: challengeRepo,
		},
	}
	service := newStepUpTestService(uow)

	result, err := service.BeginStepUp(context.Background(), StepUpBeginCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		SessionID:      "sess-1",
		OperationScope: "  sessions:read  ",
		ApplicationID:  "app-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "step-up-challenge-id", result.ChallengeID)
	assert.Equal(t, "sessions:read", result.OperationScope)
	assert.Equal(t, stepUpTestNow.Add(StepUpChallengeTTL), result.ExpiresAt)

	challenge := challengeRepo.items["step-up-challenge-id"]
	require.NotNil(t, challenge)
	assert.Equal(t, "org-1", challenge.OrganizationID)
	assert.Equal(t, "user-1", challenge.UserID)
	assert.Equal(t, "app-1", challenge.ApplicationID)
	assert.Equal(t, mfachallengedomain.StatePending, challenge.State)
	assert.Equal(t, StepUpMethodType, challenge.MethodType)
	assert.Equal(t, int64(1), challenge.Revision)
	assert.Equal(t, stepUpTestNow.Add(StepUpChallengeTTL), challenge.ExpiresAt)
	assert.True(t, stepUpChallengeMatches(challenge, "sess-1", "sessions:read"))
}

func TestStepUpChallengeMatches(t *testing.T) {
	challenge := &mfachallengedomain.Challenge{
		Metadata: map[string]any{
			StepUpMetadataSessionID:      "sess-1",
			StepUpMetadataOperationScope: "sessions:write",
		},
	}
	assert.True(t, stepUpChallengeMatches(challenge, "sess-1", "sessions:write"))
	assert.False(t, stepUpChallengeMatches(challenge, "sess-2", "sessions:write"))
	assert.False(t, stepUpChallengeMatches(challenge, "sess-1", "sessions:read"))
	assert.False(t, stepUpChallengeMatches(challenge, "sess-1", ""))

	missing := &mfachallengedomain.Challenge{Metadata: map[string]any{}}
	assert.False(t, stepUpChallengeMatches(missing, "sess-1", "sessions:write"))
	assert.False(t, stepUpChallengeMatches(nil, "sess-1", "sessions:write"))
}

func TestFinishStepUpRejectsInvalidChallenges(t *testing.T) {
	now := stepUpTestNow

	pending := &mfachallengedomain.Challenge{
		ID:             "challenge-1",
		OrganizationID: "org-1",
		UserID:         "user-1",
		State:          mfachallengedomain.StatePending,
		MethodType:     StepUpMethodType,
		Metadata: map[string]any{
			StepUpMetadataSessionID:      "sess-1",
			StepUpMetadataOperationScope: "sessions:write",
		},
		ExpiresAt: now.Add(StepUpChallengeTTL),
	}

	cases := []struct {
		name      string
		challenge *mfachallengedomain.Challenge
		command   StepUpFinishCommand
	}{
		{
			name:      "missing challenge",
			challenge: nil,
			command:   StepUpFinishCommand{OrganizationID: "org-1", UserID: "user-1", SessionID: "sess-1", ChallengeID: "unknown", OperationScope: "sessions:write", Passcode: "123456"},
		},
		{
			name:      "expired challenge",
			challenge: withStepUpExpiry(pending, now.Add(-time.Minute)),
			command:   StepUpFinishCommand{OrganizationID: "org-1", UserID: "user-1", SessionID: "sess-1", ChallengeID: "challenge-1", OperationScope: "sessions:write", Passcode: "123456"},
		},
		{
			name:      "already consumed challenge",
			challenge: withStepUpState(pending, mfachallengedomain.StateApproved),
			command:   StepUpFinishCommand{OrganizationID: "org-1", UserID: "user-1", SessionID: "sess-1", ChallengeID: "challenge-1", OperationScope: "sessions:write", Passcode: "123456"},
		},
		{
			name:      "cross session",
			challenge: pending,
			command:   StepUpFinishCommand{OrganizationID: "org-1", UserID: "user-1", SessionID: "sess-2", ChallengeID: "challenge-1", OperationScope: "sessions:write", Passcode: "123456"},
		},
		{
			name:      "cross scope",
			challenge: pending,
			command:   StepUpFinishCommand{OrganizationID: "org-1", UserID: "user-1", SessionID: "sess-1", ChallengeID: "challenge-1", OperationScope: "sessions:read", Passcode: "123456"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := map[string]*mfachallengedomain.Challenge{}
			if tc.challenge != nil {
				items[tc.challenge.ID] = tc.challenge
			}
			challengeRepo := &loginTestChallengeRepository{items: items}
			uow := stepUpTestUnitOfWork{
				loginTestUnitOfWork: loginTestUnitOfWork{
					userRepo:      &loginTestUserRepository{},
					challengeRepo: challengeRepo,
				},
			}
			service := newStepUpTestService(uow)

			_, err := service.FinishStepUp(context.Background(), tc.command)
			require.Error(t, err)
			var appErr *apperrors.Error
			require.True(t, errors.As(err, &appErr))
			assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)
			assert.Equal(t, "step-up challenge invalid or expired", appErr.Message)
		})
	}
}

func TestFinishStepUpRejectsMissingTotp(t *testing.T) {
	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{
		"challenge-1": {
			ID:             "challenge-1",
			OrganizationID: "org-1",
			UserID:         "user-1",
			State:          mfachallengedomain.StatePending,
			MethodType:     StepUpMethodType,
			Metadata: map[string]any{
				StepUpMetadataSessionID:      "sess-1",
				StepUpMetadataOperationScope: "sessions:write",
			},
			ExpiresAt: stepUpTestNow.Add(StepUpChallengeTTL),
		},
	}}
	userRepo := &loginTestUserRepository{users: map[string]*userdomain.User{
		"user-1": {ID: "user-1", OrganizationID: "org-1", Revision: 1},
	}}
	uow := stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{userRepo: userRepo, challengeRepo: challengeRepo},
	}
	service := newStepUpTestService(uow)

	_, err := service.FinishStepUp(context.Background(), StepUpFinishCommand{
		OrganizationID: "org-1", UserID: "user-1", SessionID: "sess-1",
		ChallengeID: "challenge-1", OperationScope: "sessions:write", Passcode: "123456",
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "totp is not configured", appErr.Message)
}

func TestFinishStepUpElevatesOnlyCurrentSession(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	passcode, err := totp.GenerateCode(secret, stepUpTestNow)
	require.NoError(t, err)

	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{
		"challenge-1": {
			ID:             "challenge-1",
			OrganizationID: "org-1",
			UserID:         "user-1",
			State:          mfachallengedomain.StatePending,
			MethodType:     StepUpMethodType,
			Metadata: map[string]any{
				StepUpMetadataSessionID:      "sess-1",
				StepUpMetadataOperationScope: "sessions:write",
			},
			ExpiresAt: stepUpTestNow.Add(StepUpChallengeTTL),
			Revision:  1,
		},
	}}
	userRepo := &loginTestUserRepository{
		revisionGuard: true,
		users: map[string]*userdomain.User{
			"user-1": {
				ID:             "user-1",
				OrganizationID: "org-1",
				Revision:       1,
				TotpSecret:     secret,
				MfaMethods: []userdomain.UserMfaMethod{
					{MethodType: mfaTypeTotp, Enabled: true, Verified: true},
				},
			},
		},
	}
	sessionRepo := &stepUpTestSessionRepository{
		loginTestSessionRepository: &loginTestSessionRepository{sessions: map[string]*sessiondomain.Session{
			"sess-1": {
				ID: "sess-1", UserID: "user-1", OrganizationID: "org-1",
				SessionState: sessiondomain.StateActive, ExpiresAt: stepUpTestNow.Add(time.Hour),
				Revision: 1, AuthenticationLevel: "password", AssuranceLevel: sessiondomain.AssuranceLevelAAL1,
			},
			"sess-2": {
				ID: "sess-2", UserID: "user-1", OrganizationID: "org-1",
				SessionState: sessiondomain.StateActive, ExpiresAt: stepUpTestNow.Add(time.Hour),
				Revision: 1, AuthenticationLevel: "password", AssuranceLevel: sessiondomain.AssuranceLevelAAL1,
			},
		}},
	}
	uow := stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{userRepo: userRepo, challengeRepo: challengeRepo},
		sessions:            sessionRepo,
	}
	service := newStepUpTestService(uow)

	result, err := service.FinishStepUp(context.Background(), StepUpFinishCommand{
		OrganizationID: "org-1", UserID: "user-1", SessionID: "sess-1",
		ChallengeID: "challenge-1", OperationScope: " sessions:write ", Passcode: passcode,
	})
	require.NoError(t, err)
	assert.Equal(t, sessiondomain.AssuranceLevelAAL2, result.AssuranceLevel)
	assert.Equal(t, "sess-1", result.SessionID)
	assert.Equal(t, "sessions:write", result.OperationScope)
	assert.Equal(t, stepUpTestNow, result.ElevatedAt)

	// Only the current session is updated, and it carries AAL2 afterwards.
	require.Len(t, sessionRepo.updated, 1)
	assert.Equal(t, "sess-1", sessionRepo.updated[0].ID)
	assert.Equal(t, sessiondomain.AssuranceLevelAAL2, sessionRepo.updated[0].AssuranceLevel)
	assert.Equal(t, int64(2), sessionRepo.updated[0].Revision)
	assert.Equal(t, sessiondomain.AssuranceLevelAAL1, sessionRepo.sessions["sess-2"].AssuranceLevel)
	assert.Equal(t, int64(1), sessionRepo.sessions["sess-2"].Revision)

	assert.Equal(t, mfachallengedomain.StateApproved, challengeRepo.items["challenge-1"].State)
	assert.Equal(t, int64(2), userRepo.users["user-1"].Revision)
	require.NotNil(t, userRepo.users["user-1"].MfaMethods[0].LastUsedAt)
}

func withStepUpExpiry(challenge *mfachallengedomain.Challenge, expiresAt time.Time) *mfachallengedomain.Challenge {
	cloned := *challenge
	cloned.ExpiresAt = expiresAt
	return &cloned
}

func withStepUpState(challenge *mfachallengedomain.Challenge, state string) *mfachallengedomain.Challenge {
	cloned := *challenge
	cloned.State = state
	return &cloned
}
