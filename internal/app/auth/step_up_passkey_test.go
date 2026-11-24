package auth

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webauthnmock "github.com/guliangiam/guliangiam/internal/domain/webauthn/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
)

const (
	stepUpPasskeyChallengeID = "019chal-0000-0000-0000-0000000000pk"
	stepUpPasskeyPlain       = "step-up-passkey-challenge"
	stepUpPasskeyRPID        = "login.example.com"
)

func passkeyStepUpOrg(secondFactor bool) *orgdomain.Organization {
	return newTestOrg(func(org *orgdomain.Organization) {
		org.Security.MfaPolicy = map[string]any{
			platformwebauthn.PolicyKeyPasskeyEnabled: true,
			policyKeyPasskeyAsSecondFactor:           secondFactor,
		}
	})
}

func passkeyStepUpChallenge(org *orgdomain.Organization, user *userdomain.User, sessionID, scope string) *mfachallengedomain.Challenge {
	return &mfachallengedomain.Challenge{
		ID:             "challenge-1",
		OrganizationID: org.ID,
		UserID:         user.ID,
		State:          mfachallengedomain.StatePending,
		MethodType:     StepUpMethodType,
		Metadata: map[string]any{
			StepUpMetadataSessionID:      sessionID,
			StepUpMetadataOperationScope: scope,
		},
		ExpiresAt: stepUpTestNow.Add(StepUpChallengeTTL),
		Revision:  1,
	}
}

func passkeyStepUpWebAuthnChallenge(org *orgdomain.Organization, user *userdomain.User, plain string) *webauthndomain.Challenge {
	return &webauthndomain.Challenge{
		ID:             stepUpPasskeyChallengeID,
		ChallengeHash:  webauthndomain.HashChallenge(plain),
		CeremonyType:   webauthndomain.CeremonyTypeAuthentication,
		UserID:         user.ID,
		OrganizationID: org.ID,
		RelyingPartyID: stepUpPasskeyRPID,
		ExpiresAt:      stepUpTestNow.Add(time.Minute),
	}
}

func passkeyStepUpSessions() *stepUpTestSessionRepository {
	return &stepUpTestSessionRepository{
		loginTestSessionRepository: &loginTestSessionRepository{sessions: map[string]*sessiondomain.Session{
			"sess-1": {
				ID: "sess-1", UserID: testUserID, OrganizationID: testOrgID,
				SessionState: sessiondomain.StateActive, ExpiresAt: stepUpTestNow.Add(time.Hour),
				Revision: 1, AuthenticationLevel: "password", AssuranceLevel: sessiondomain.AssuranceLevelAAL1,
			},
			"sess-2": {
				ID: "sess-2", UserID: testUserID, OrganizationID: testOrgID,
				SessionState: sessiondomain.StateActive, ExpiresAt: stepUpTestNow.Add(time.Hour),
				Revision: 1, AuthenticationLevel: "password", AssuranceLevel: sessiondomain.AssuranceLevelAAL1,
			},
		}},
	}
}

func stepUpPasskeyCommand(challenge, signature, authenticatorData, clientDataJSON, credentialID string) StepUpFinishCommand {
	return StepUpFinishCommand{
		OrganizationID:      testOrgID,
		UserID:              testUserID,
		SessionID:           "sess-1",
		ChallengeID:         "challenge-1",
		OperationScope:      "sessions:write",
		WebAuthnChallengeID: stepUpPasskeyChallengeID,
		WebAuthnChallenge:   challenge,
		CredentialID:        base64.RawURLEncoding.EncodeToString([]byte(credentialID)),
		Signature:           signature,
		AuthenticatorData:   authenticatorData,
		ClientDataJSON:      clientDataJSON,
	}
}

func TestFinishStepUpWithPasskeyElevatesOnlyCurrentSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := passkeyStepUpOrg(true)
	user := newTestUser()

	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, stepUpPasskeyRPID, stepUpPasskeyPlain, 1)
	credential := activeCredential(user.ID, coseKey)

	repo := webauthnmock.NewMockRepository(ctrl)
	repo.EXPECT().ConsumeChallenge(gomock.Any(), stepUpPasskeyChallengeID, gomock.Any()).
		Return(passkeyStepUpWebAuthnChallenge(org, user, stepUpPasskeyPlain), nil)
	repo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	repo.EXPECT().UpdateSignCount(gomock.Any(), credential.ID, int64(1)).Return(nil)

	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{
		"challenge-1": passkeyStepUpChallenge(org, user, "sess-1", "sessions:write"),
	}}
	sessionRepo := passkeyStepUpSessions()
	svc := newStepUpTestService(stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			orgRepo:       newQRTestOrgRepository(org),
			userRepo:      &loginTestUserRepository{users: map[string]*userdomain.User{user.ID: user}},
			challengeRepo: challengeRepo,
			webAuthnRepo:  repo,
		},
		sessions: sessionRepo,
	})

	result, err := svc.FinishStepUp(context.Background(), stepUpPasskeyCommand(stepUpPasskeyPlain, signature, authData, clientData, string(credential.CredentialID)))
	require.NoError(t, err)
	assert.Equal(t, sessiondomain.AssuranceLevelAAL2, result.AssuranceLevel)
	assert.Equal(t, "sess-1", result.SessionID)
	assert.Equal(t, "sessions:write", result.OperationScope)

	require.Len(t, sessionRepo.updated, 1)
	assert.Equal(t, "sess-1", sessionRepo.updated[0].ID)
	assert.Equal(t, sessiondomain.AssuranceLevelAAL2, sessionRepo.updated[0].AssuranceLevel)
	assert.Equal(t, int64(2), sessionRepo.updated[0].Revision)
	// Cross-session elevation is impossible: the other session is untouched.
	assert.Equal(t, sessiondomain.AssuranceLevelAAL1, sessionRepo.sessions["sess-2"].AssuranceLevel)
	assert.Equal(t, int64(1), sessionRepo.sessions["sess-2"].Revision)

	assert.Equal(t, mfachallengedomain.StateApproved, challengeRepo.items["challenge-1"].State)
}

func TestFinishStepUpWithPasskeyRejectsInvalidAssertion(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := passkeyStepUpOrg(true)
	user := newTestUser()

	// Assertion signed by key A; the stored credential carries unrelated key B.
	clientData, authData, signature, _ := signWebAuthnAssertion(t, stepUpPasskeyRPID, stepUpPasskeyPlain, 1)
	_, _, _, foreignKey := signWebAuthnAssertion(t, stepUpPasskeyRPID, "another-challenge", 1)
	credential := activeCredential(user.ID, foreignKey)

	repo := webauthnmock.NewMockRepository(ctrl)
	repo.EXPECT().ConsumeChallenge(gomock.Any(), stepUpPasskeyChallengeID, gomock.Any()).
		Return(passkeyStepUpWebAuthnChallenge(org, user, stepUpPasskeyPlain), nil)
	repo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	// No UpdateSignCount / no session update: the rejection happens before writes.

	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{
		"challenge-1": passkeyStepUpChallenge(org, user, "sess-1", "sessions:write"),
	}}
	sessionRepo := passkeyStepUpSessions()
	svc := newStepUpTestService(stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			orgRepo:       newQRTestOrgRepository(org),
			userRepo:      &loginTestUserRepository{users: map[string]*userdomain.User{user.ID: user}},
			challengeRepo: challengeRepo,
			webAuthnRepo:  repo,
		},
		sessions: sessionRepo,
	})

	_, err := svc.FinishStepUp(context.Background(), stepUpPasskeyCommand(stepUpPasskeyPlain, signature, authData, clientData, string(credential.CredentialID)))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	assert.Empty(t, sessionRepo.updated)
	assert.Equal(t, mfachallengedomain.StatePending, challengeRepo.items["challenge-1"].State)
}

func TestFinishStepUpWithPasskeyRejectsReplayedStepUpChallenge(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := passkeyStepUpOrg(true)
	user := newTestUser()

	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, stepUpPasskeyRPID, stepUpPasskeyPlain, 1)
	credential := activeCredential(user.ID, coseKey)

	repo := webauthnmock.NewMockRepository(ctrl)
	repo.EXPECT().ConsumeChallenge(gomock.Any(), stepUpPasskeyChallengeID, gomock.Any()).
		Return(passkeyStepUpWebAuthnChallenge(org, user, stepUpPasskeyPlain), nil)
	repo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	repo.EXPECT().UpdateSignCount(gomock.Any(), credential.ID, int64(1)).Return(nil)

	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{
		"challenge-1": passkeyStepUpChallenge(org, user, "sess-1", "sessions:write"),
	}}
	sessionRepo := passkeyStepUpSessions()
	svc := newStepUpTestService(stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			orgRepo:       newQRTestOrgRepository(org),
			userRepo:      &loginTestUserRepository{users: map[string]*userdomain.User{user.ID: user}},
			challengeRepo: challengeRepo,
			webAuthnRepo:  repo,
		},
		sessions: sessionRepo,
	})

	command := stepUpPasskeyCommand(stepUpPasskeyPlain, signature, authData, clientData, string(credential.CredentialID))
	_, err := svc.FinishStepUp(context.Background(), command)
	require.NoError(t, err)

	// Same step-up challenge again: already approved, so it is rejected before
	// the WebAuthn challenge is even looked at (no extra mock expectation).
	_, replayErr := svc.FinishStepUp(context.Background(), command)
	require.Error(t, replayErr)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(replayErr).Code)
	assert.Len(t, sessionRepo.updated, 1)
}

func TestFinishStepUpWithPasskeyRejectsReplayedWebAuthnChallenge(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := passkeyStepUpOrg(true)
	user := newTestUser()

	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, stepUpPasskeyRPID, stepUpPasskeyPlain, 1)
	credential := activeCredential(user.ID, coseKey)

	repo := webauthnmock.NewMockRepository(ctrl)
	// A fresh step-up challenge still points at an already consumed WebAuthn challenge.
	repo.EXPECT().ConsumeChallenge(gomock.Any(), stepUpPasskeyChallengeID, gomock.Any()).
		Return(nil, apperrors.NewNotFound("webauthn challenge already consumed"))

	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{
		"challenge-1": passkeyStepUpChallenge(org, user, "sess-1", "sessions:write"),
	}}
	sessionRepo := passkeyStepUpSessions()
	svc := newStepUpTestService(stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			orgRepo:       newQRTestOrgRepository(org),
			userRepo:      &loginTestUserRepository{users: map[string]*userdomain.User{user.ID: user}},
			challengeRepo: challengeRepo,
			webAuthnRepo:  repo,
		},
		sessions: sessionRepo,
	})

	_, err := svc.FinishStepUp(context.Background(), stepUpPasskeyCommand(stepUpPasskeyPlain, signature, authData, clientData, string(credential.CredentialID)))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeGone, apperrors.FromError(err).Code)
	assert.Empty(t, sessionRepo.updated)
}

func TestFinishStepUpWithPasskeyRejectsCrossSessionElevation(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := passkeyStepUpOrg(true)
	user := newTestUser()

	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, stepUpPasskeyRPID, stepUpPasskeyPlain, 1)
	credential := activeCredential(user.ID, coseKey)

	// The challenge belongs to sess-1; the request claims sess-2. No WebAuthn
	// repository expectation: the step-up binding must reject before any consume.
	repo := webauthnmock.NewMockRepository(ctrl)
	_ = repo

	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{
		"challenge-1": passkeyStepUpChallenge(org, user, "sess-1", "sessions:write"),
	}}
	sessionRepo := passkeyStepUpSessions()
	svc := newStepUpTestService(stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			orgRepo:       newQRTestOrgRepository(org),
			userRepo:      &loginTestUserRepository{users: map[string]*userdomain.User{user.ID: user}},
			challengeRepo: challengeRepo,
			webAuthnRepo:  repo,
		},
		sessions: sessionRepo,
	})

	command := stepUpPasskeyCommand(stepUpPasskeyPlain, signature, authData, clientData, string(credential.CredentialID))
	command.SessionID = "sess-2"

	_, err := svc.FinishStepUp(context.Background(), command)
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnauthorized, apperrors.FromError(err).Code)
	assert.Empty(t, sessionRepo.updated)
	assert.Equal(t, mfachallengedomain.StatePending, challengeRepo.items["challenge-1"].State)
}

func TestFinishStepUpWithPasskeyRejectsDisabledPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := passkeyStepUpOrg(false)
	user := newTestUser()

	repo := webauthnmock.NewMockRepository(ctrl)
	// No ConsumeChallenge expectation: the policy gate must win first.
	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{
		"challenge-1": passkeyStepUpChallenge(org, user, "sess-1", "sessions:write"),
	}}
	sessionRepo := passkeyStepUpSessions()
	svc := newStepUpTestService(stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			orgRepo:       newQRTestOrgRepository(org),
			userRepo:      &loginTestUserRepository{users: map[string]*userdomain.User{user.ID: user}},
			challengeRepo: challengeRepo,
			webAuthnRepo:  repo,
		},
		sessions: sessionRepo,
	})

	_, err := svc.FinishStepUp(context.Background(), stepUpPasskeyCommand(stepUpPasskeyPlain, "sig", "auth", "client", "credential-id-001"))
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeForbidden, apperrors.FromError(err).Code)
	assert.Empty(t, sessionRepo.updated)
}

func TestFinishStepUpRequiresPasscodeOrAssertion(t *testing.T) {
	challengeRepo := &loginTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{
		"challenge-1": {
			ID: "challenge-1", OrganizationID: testOrgID, UserID: testUserID,
			State: mfachallengedomain.StatePending, MethodType: StepUpMethodType,
			Metadata: map[string]any{
				StepUpMetadataSessionID:      "sess-1",
				StepUpMetadataOperationScope: "sessions:write",
			},
			ExpiresAt: stepUpTestNow.Add(StepUpChallengeTTL),
		},
	}}
	svc := newStepUpTestService(stepUpTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			userRepo:      &loginTestUserRepository{},
			challengeRepo: challengeRepo,
		},
	})

	_, err := svc.FinishStepUp(context.Background(), StepUpFinishCommand{
		OrganizationID: testOrgID, UserID: testUserID, SessionID: "sess-1",
		ChallengeID: "challenge-1", OperationScope: "sessions:write",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
}
