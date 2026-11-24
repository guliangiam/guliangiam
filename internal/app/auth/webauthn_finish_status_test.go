package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webauthnmock "github.com/guliangiam/guliangiam/internal/domain/webauthn/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 挑战缺失 / 已消费 / 已过期在仓储层都表现为 NotFound。服务层必须统一映射为
// 410 Gone 且响应消息完全一致，调用方无法据此枚举挑战是否曾经存在。
func TestWebAuthnRegistrationFinish_ChallengeGoneIsIndistinguishable410(t *testing.T) {
	scenarios := []struct {
		name string
		err  error
	}{
		{name: "challenge_missing", err: apperrors.NewNotFound("challenge row missing")},
		{name: "challenge_consumed", err: apperrors.NewNotFound("challenge already consumed")},
		{name: "challenge_expired", err: apperrors.NewNotFound("challenge expired")},
	}

	messages := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := webauthnmock.NewMockRepository(ctrl)
			repo.EXPECT().ConsumeChallenge(gomock.Any(), testChallengeID, gomock.Any()).Return(nil, scenario.err)
			svc := newTestService(loginTestUnitOfWork{webAuthnRepo: repo})

			_, _, err := svc.WebAuthnRegistrationFinish(context.Background(), WebAuthnRegistrationFinishCommand{
				OrganizationSlug: testOrgSlug,
				Username:         testUsername,
				ChallengeID:      testChallengeID,
				Challenge:        "client-challenge",
				CredentialID:     "Y3JlZA",
				PublicKey:        "cHVibGljLWtleQ",
			})
			require.Error(t, err)
			appErr := apperrors.FromError(err)
			assert.Equal(t, apperrors.CodeGone, appErr.Code)
			messages = append(messages, appErr.Message)
		})
	}

	require.Len(t, messages, len(scenarios))
	for i := 1; i < len(messages); i++ {
		assert.Equal(t, messages[0], messages[i], "410 响应消息必须不可区分")
	}
}

// 挑战明文与 begin 下发的不一致属于校验失败，必须是 422；且发生在任何凭据查询之前。
func TestWebAuthnRegistrationFinish_ChallengeMismatchReturns422(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := webauthnmock.NewMockRepository(ctrl)
	user := newTestUser()
	org := newTestOrg()
	stubIssuedChallenge(t, repo, webauthndomain.CeremonyTypeRegistration, "server-issued-challenge", user.ID, org.Slug)
	svc := newTestService(loginTestUnitOfWork{webAuthnRepo: repo})

	_, _, err := svc.WebAuthnRegistrationFinish(context.Background(), WebAuthnRegistrationFinishCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ChallengeID:      testChallengeID,
		Challenge:        "client-supplied-challenge",
		CredentialID:     "Y3JlZA",
		PublicKey:        "cHVibGljLWtleQ",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
}

// attestation 验签/解析失败同样属于校验失败，必须是 422，而不是 401。
func TestWebAuthnRegistrationFinish_AttestationFailureReturns422(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := webauthnmock.NewMockRepository(ctrl)
	user := newTestUser()
	org := newTestOrg()
	stubIssuedChallenge(t, repo, webauthndomain.CeremonyTypeRegistration, "server-issued-challenge", user.ID, org.Slug)

	uow := newTestUow(org, user, newTestApp())
	uow.webAuthnRepo = repo
	svc := newTestService(uow)

	_, _, err := svc.WebAuthnRegistrationFinish(context.Background(), WebAuthnRegistrationFinishCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ChallengeID:      testChallengeID,
		Challenge:        "server-issued-challenge",
		CredentialID:     "Y3JlZA",
		PublicKey:        "cHVibGljLWtleQ",
		ClientDataJSON:   "",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	assert.Contains(t, apperrors.FromError(err).Message, "verification failed")
}
