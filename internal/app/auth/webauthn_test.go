package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/guliangiam/guliangiam/internal/domain/applications"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webauthnmock "github.com/guliangiam/guliangiam/internal/domain/webauthn/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// WebAuthn 登录（《开发设计》§45）服务层用例：断言校验通过后必须签发会话、撤销请求
// 携带的旧会话，并守住「凭据属主 = 命令里的用户」这条边界。
//
// 断言字段由真实 ES256 私钥生成，不 stub VerifyAssertion —— 密码学路径一旦用假实现
// 顶替，测试就不再证明任何事。

// testChallengeID 是 finish 用例里 begin 返回的挑战句柄（仓储由用例自己 stub）。
const testChallengeID = "019chal-0000-0000-0000-000000000001"

// defaultConsoleApp 构造组织默认控制台应用（app-<orgSlug>）：普通控制台登录
// 不携带 applicationId，finish 用它补全会话的 application_id 外键。
func defaultConsoleApp(orgID, orgSlug string) *applications.Application {
	return &applications.Application{ID: "019app-default-0000-0000-000000000001", OrganizationID: orgID, Slug: "app-" + orgSlug}
}

// stubIssuedChallenge 让挑战仓储在 finish 时原子消费成功，返回 begin 时签发的那一行。
// 哈希与 rpId 都按真实口径填，因此「客户端回传明文 → 重算哈希比对」这条路径是真的在跑。
func stubIssuedChallenge(t *testing.T, repo *webauthnmock.MockRepository, ceremony, challenge, userID, relyingPartyID string) {
	t.Helper()
	repo.EXPECT().ConsumeChallenge(gomock.Any(), testChallengeID, gomock.Any()).Return(&webauthndomain.Challenge{
		ID:             testChallengeID,
		ChallengeHash:  webauthndomain.HashChallenge(challenge),
		CeremonyType:   ceremony,
		UserID:         userID,
		RelyingPartyID: relyingPartyID,
		ExpiresAt:      time.Now().Add(time.Minute),
		CreatedAt:      time.Now().Add(-time.Second),
		UpdatedAt:      time.Now(),
		Revision:       1,
	}, nil)
}

// signWebAuthnAssertion 生成一次合法断言所需的三个 base64url 字段，以及对应的
// ES256 COSE 公钥（来源取用例约定的控制台来源）。
func signWebAuthnAssertion(t *testing.T, relyingPartyID, challenge string, signCount uint32) (clientDataJSON, authenticatorData, signature string, coseKey []byte) {
	t.Helper()
	return signWebAuthnAssertionWithOrigin(t, relyingPartyID, challenge, "https://login.example.com", signCount)
}

// signWebAuthnAssertionWithOrigin 同上，但指定 clientDataJSON 里的 origin——用于验证
// 「签名本身合法、来源却不是我们的站点」这条攻击路径。
func signWebAuthnAssertionWithOrigin(t *testing.T, relyingPartyID, challenge, origin string, signCount uint32) (clientDataJSON, authenticatorData, signature string, coseKey []byte) {
	t.Helper()
	return signWebAuthnAssertionFull(t, relyingPartyID, challenge, origin, 0x01, signCount)
}

// signWebAuthnAssertionFull 同上，但可指定 authenticatorData 的 flags 字节：
// 0x01 = UP、0x05 = UP+UV（VerifyAssertion 要求 UP 必须置位）。
func signWebAuthnAssertionFull(t *testing.T, relyingPartyID, challenge, origin string, flags byte, signCount uint32) (clientDataJSON, authenticatorData, signature string, coseKey []byte) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	rpIDHash := sha256.Sum256([]byte(relyingPartyID))
	authData := make([]byte, 0, 37)
	authData = append(authData, rpIDHash[:]...)
	authData = append(authData, flags)
	authData = append(authData, byte(signCount>>24), byte(signCount>>16), byte(signCount>>8), byte(signCount))

	// VerifyAssertion 把存储的挑战字符串当原始字节再做 base64url 编码比对。
	clientData := fmt.Sprintf(
		"{\"type\":\"webauthn.get\",\"challenge\":\"%s\",\"origin\":\"%s\"}",
		base64.RawURLEncoding.EncodeToString([]byte(challenge)),
		origin,
	)
	clientDataHash := sha256.Sum256([]byte(clientData))
	digest := sha256.Sum256(append(append([]byte{}, authData...), clientDataHash[:]...))

	rawSignature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	require.NoError(t, err)

	return base64.RawURLEncoding.EncodeToString([]byte(clientData)),
		base64.RawURLEncoding.EncodeToString(authData),
		base64.RawURLEncoding.EncodeToString(rawSignature),
		coseES256PublicKey(privateKey.PublicKey)
}

// coseES256PublicKey 手写 ES256 COSE_Key：{1:2, 3:-7, -1:1, -2:x, -3:y}。
func coseES256PublicKey(publicKey ecdsa.PublicKey) []byte {
	x := publicKey.X.FillBytes(make([]byte, 32))
	y := publicKey.Y.FillBytes(make([]byte, 32))
	key := []byte{0xa5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}
	key = append(key, x...)
	key = append(key, 0x22, 0x58, 0x20)
	key = append(key, y...)
	return key
}

// activeCredential 构造一个属于 userID 的可用 passkey 凭据。
func activeCredential(userID string, publicKey []byte) *webauthndomain.Credential {
	return &webauthndomain.Credential{
		ID:              "019cred-0000-0000-0000-000000000001",
		UserID:          userID,
		CredentialID:    []byte("credential-id-001"),
		PublicKey:       publicKey,
		AttestationType: "none",
		SignCount:       0,
		Status:          webauthndomain.CredentialStatusActive,
	}
}

func TestWebAuthnAuthenticationFinish_IssuesSessionAndConsumesChallenge(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, org.Slug, challenge, 1)
	credential := activeCredential(user.ID, coseKey)

	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	credentialRepo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	credentialRepo.EXPECT().UpdateSignCount(gomock.Any(), credential.ID, int64(1)).Return(nil)

	sessionRepo := &loginTestSessionRepository{}
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     sessionRepo,
		webAuthnRepo: credentialRepo,
	})

	command := WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  org.Slug,
		Username:          user.Username,
		ChallengeID:       testChallengeID,
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(credential.CredentialID),
		Signature:         signature,
		AuthenticatorData: authData,
		ClientDataJSON:    clientData,
		PreviousSessionID: "old-session",
		IPAddress:         "203.0.113.7",
		UserAgent:         "test-agent",
	}

	result, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), command)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, sessionID, "登录态必须在 finish 签发")
	assert.Equal(t, sessionID, result.SessionID)
	assert.Equal(t, "success", result.Response.Status)

	issued := sessionRepo.sessions[sessionID]
	require.NotNil(t, issued)
	assert.Equal(t, user.ID, issued.UserID)
	assert.Equal(t, org.ID, issued.OrganizationID)
	assert.Equal(t, "webauthn", issued.DeviceInfo["loginMethod"])
	assert.Equal(t, []string{"old-session"}, sessionRepo.revoked, "S9：旧会话必须被撤销")
	assert.Equal(t, "203.0.113.7", issued.IPAddress, "会话元数据由服务端解析后落库")
	assert.Equal(t, "test-agent", issued.UserAgent)
	require.NotNil(t, user.LastSigninAt, "服务端登录路径同样要记录 signin 统计")
	assert.Equal(t, "203.0.113.7", user.LastSigninIP)

	// 挑战一次性：同一个 challengeId 再 finish 一次必须是 410，且不再签发会话。
	// 第二次消费对应「条件 UPDATE 没抢到行」——仓储返回 NotFound，而不是同一行被消费两次。
	credentialRepo.EXPECT().ConsumeChallenge(gomock.Any(), testChallengeID, gomock.Any()).
		Return(nil, apperrors.NewNotFound("webauthn challenge already consumed")).
		Times(1)
	_, replayedSessionID, replayErr := svc.WebAuthnAuthenticationFinish(context.Background(), command)
	require.Error(t, replayErr)
	assert.Equal(t, apperrors.CodeGone, apperrors.FromError(replayErr).Code)
	assert.Empty(t, replayedSessionID)
	assert.Len(t, sessionRepo.sessions, 1)
}

// 计数器恒为 0 的认证器（规范允许不上报签名计数）不得被单调性写入误判为回退。
func TestWebAuthnAuthenticationFinish_AcceptsZeroSignCountAuthenticator(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	// stored=0、received=0：0/0 组合必须跳过回退判定与写回。
	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, org.Slug, challenge, 0)
	credential := activeCredential(user.ID, coseKey)
	credential.SignCount = 0

	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	credentialRepo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	// UpdateSignCount 不得被调用：0/0 在仓储层会命中 0 行并被判成 409。

	sessionRepo := &loginTestSessionRepository{}
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     sessionRepo,
		webAuthnRepo: credentialRepo,
	})

	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  org.Slug,
		Username:          user.Username,
		ChallengeID:       testChallengeID,
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(credential.CredentialID),
		Signature:         signature,
		AuthenticatorData: authData,
		ClientDataJSON:    clientData,
	})
	require.NoError(t, err)
	require.NotEmpty(t, sessionID, "0/0 认证器必须能正常登录")
	assert.Len(t, sessionRepo.sessions, 1)
}

// 真正的计数器回退（received>0 且 < stored）仍必须被拦截并撤销凭证。
func TestWebAuthnAuthenticationFinish_RejectsSignCountRegression(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, org.Slug, challenge, 3)
	credential := activeCredential(user.ID, coseKey)
	credential.SignCount = 5

	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	credentialRepo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	credentialRepo.EXPECT().
		UpdateStatus(gomock.Any(), credential.ID, webauthndomain.CredentialStatusRevoked).
		Return(nil)
	// UpdateSignCount 不得被调用：回退必须在任何写入之前被拒绝。

	sessionRepo := &loginTestSessionRepository{}
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     sessionRepo,
		webAuthnRepo: credentialRepo,
	})

	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  org.Slug,
		Username:          user.Username,
		ChallengeID:       testChallengeID,
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(credential.CredentialID),
		Signature:         signature,
		AuthenticatorData: authData,
		ClientDataJSON:    clientData,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	assert.Contains(t, apperrors.FromError(err).Message, "revoked")
	assert.Empty(t, sessionID)
	assert.Empty(t, sessionRepo.sessions)
}

func TestWebAuthnAuthenticationFinish_AppliesOrganizationSessionPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	// 组织策略为 reject_new 且已有一个活跃会话：finish 必须失败（此前三条服务端登录
	// 路径完全不看会话策略，同一租户的策略会随「点哪个登录按钮」而变）。
	org := newConcurrentPolicyOrg(1, "reject_new")
	user := newTestUser()
	challenge := "server-issued-challenge"

	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, org.Slug, challenge, 1)
	credential := activeCredential(user.ID, coseKey)

	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	credentialRepo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	credentialRepo.EXPECT().UpdateSignCount(gomock.Any(), credential.ID, int64(1)).Return(nil)

	existing := newPolicySession("019ses-existing", user.ID, "203.0.113.9", time.Now().Add(-time.Hour))
	sessionRepo := &loginTestSessionRepository{
		sessions: map[string]*sessiondomain.Session{existing.ID: existing},
	}
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     sessionRepo,
		webAuthnRepo: credentialRepo,
	})

	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  org.Slug,
		Username:          user.Username,
		ChallengeID:       testChallengeID,
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(credential.CredentialID),
		Signature:         signature,
		AuthenticatorData: authData,
		ClientDataJSON:    clientData,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Empty(t, sessionID, "策略拒绝时不得返回会话")
}

func TestWebAuthnAuthenticationFinish_RejectsCredentialOwnedByAnotherUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, org.Slug, challenge, 1)
	// 攻击者用自己名下的密钥对替受害者 username 完成断言：begin 对任意 username 都发
	// 挑战，若服务端不比对属主，就能拿到受害者的会话。
	foreign := activeCredential("019usr-0000-0000-0000-0000000000ff", coseKey)

	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	credentialRepo.EXPECT().GetByCredentialID(gomock.Any(), foreign.CredentialID).Return(foreign, nil)
	// UpdateSignCount 不得被调用：拒绝必须发生在任何写入之前。

	sessionRepo := &loginTestSessionRepository{}
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     sessionRepo,
		webAuthnRepo: credentialRepo,
	})

	result, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  org.Slug,
		Username:          user.Username,
		ChallengeID:       testChallengeID,
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(foreign.CredentialID),
		Signature:         signature,
		AuthenticatorData: authData,
		ClientDataJSON:    clientData,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeForbidden, apperrors.FromError(err).Code)
	assert.Nil(t, result)
	assert.Empty(t, sessionID)
	assert.Empty(t, sessionRepo.sessions)
}

func TestWebAuthnAuthenticationFinish_RejectsMismatchedChallenge(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	// 客户端回传的 challenge 与 begin 下发的不一致 → 422，且不查凭据。
	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     &loginTestSessionRepository{},
		webAuthnRepo: credentialRepo,
	})

	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug: org.Slug,
		Username:         user.Username,
		ChallengeID:      testChallengeID,
		Challenge:        "another-challenge",
		CredentialID:     base64.RawURLEncoding.EncodeToString([]byte("credential-id-001")),
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	assert.Empty(t, sessionID)
}

func TestWebAuthnAuthenticationFinish_RequiresApplicationForProtocolContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, org.Slug, challenge, 1)
	credential := activeCredential(user.ID, coseKey)

	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	credentialRepo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     &loginTestSessionRepository{},
		webAuthnRepo: credentialRepo,
	})

	// 带 responseType 却不带 applicationId：必须是 422 而不是 nil 指针 panic。
	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  org.Slug,
		Username:          user.Username,
		ChallengeID:       testChallengeID,
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(credential.CredentialID),
		Signature:         signature,
		AuthenticatorData: authData,
		ClientDataJSON:    clientData,
		ResponseType:      "code",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.Empty(t, sessionID)
}

func TestWebAuthnAuthenticationBegin_StoresOnlyTheChallengeHash(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	credentialRepo.EXPECT().ListByUserID(gomock.Any(), user.ID).
		Return([]*webauthndomain.Credential{activeCredential(user.ID, []byte("cose-key"))}, nil)

	var stored *webauthndomain.Challenge
	credentialRepo.EXPECT().CreateChallenge(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, c *webauthndomain.Challenge) error {
			stored = c
			return nil
		})

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		webAuthnRepo: credentialRepo,
	})

	begin, err := svc.WebAuthnAuthenticationBegin(context.Background(), WebAuthnAuthenticationBeginCommand{
		OrganizationSlug: org.Slug,
		Username:         user.Username,
	})
	require.NoError(t, err)
	require.NotNil(t, stored)

	// 库里只允许有哈希：明文挑战出现在响应的同时不得出现在行里（31.4 的「不存明文」）。
	assert.Equal(t, webauthndomain.HashChallenge(begin.Challenge), stored.ChallengeHash)
	assert.NotContains(t, stored.ChallengeHash, begin.Challenge)
	assert.Equal(t, webauthndomain.CeremonyTypeAuthentication, stored.CeremonyType)
	assert.Equal(t, user.ID, stored.UserID)
	assert.Equal(t, org.ID, stored.OrganizationID)
	// RP ID 从来源集合推导（不是组织 slug）：用例的允许来源是 https://login.example.com。
	assert.Equal(t, "login.example.com", stored.RelyingPartyID)
	assert.Equal(t, stored.RelyingPartyID, begin.RelyingPartyID, "响应里的 rpId 必须与落库值一致")
	assert.Equal(t, begin.ChallengeID, stored.ID)
	assert.True(t, stored.ExpiresAt.After(stored.CreatedAt), "挑战必须有 TTL")
	assert.Equal(t, "preferred", begin.UserVerification, "默认策略不强制 UV")
}

// 组织策略开启 UV 时，begin 必须把 "required" 下发，浏览器才会要求 PIN/生物识别。
func TestWebAuthnAuthenticationBegin_AdvertisesRequiredUserVerification(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	credentialRepo.EXPECT().ListByUserID(gomock.Any(), user.ID).
		Return([]*webauthndomain.Credential{activeCredential(user.ID, []byte("cose-key"))}, nil)
	credentialRepo.EXPECT().CreateChallenge(gomock.Any(), gomock.Any()).Return(nil)

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		webAuthnRepo: credentialRepo,
	})
	svc.WebAuthnRequireUV = true

	begin, err := svc.WebAuthnAuthenticationBegin(context.Background(), WebAuthnAuthenticationBeginCommand{
		OrganizationSlug: org.Slug,
		Username:         user.Username,
	})
	require.NoError(t, err)
	assert.Equal(t, "required", begin.UserVerification)
}

func TestWebAuthnAuthenticationBegin_FailsWithoutDerivableRPID(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	credentialRepo.EXPECT().ListByUserID(gomock.Any(), user.ID).
		Return([]*webauthndomain.Credential{activeCredential(user.ID, []byte("cose-key"))}, nil)
	// 两个来源分属不同可注册域 → 不存在同时覆盖两者的 RP ID，浏览器会拒绝 ceremony。
	// 必须在 begin 就失败（也不得写入挑战行——没有任何 CreateChallenge 期望）。

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		webAuthnRepo: credentialRepo,
	})
	svc.WebAuthnAllowedOrigins = []string{"https://a.example.com", "https://b.example.org"}
	svc.IssuerURL = ""

	_, err := svc.WebAuthnAuthenticationBegin(context.Background(), WebAuthnAuthenticationBeginCommand{
		OrganizationSlug: org.Slug,
		Username:         user.Username,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	assert.Contains(t, apperrors.FromError(err).Message, "RP ID")
}

func TestWebAuthnAuthenticationFinish_RejectsChallengeOfAnotherCeremony(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	// 用一个 registration 挑战来收尾认证：跨 ceremony 重放必须 403，且不查凭据。
	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeRegistration, challenge, user.ID, org.Slug)
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     &loginTestSessionRepository{},
		webAuthnRepo: credentialRepo,
	})

	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug: org.Slug,
		Username:         user.Username,
		ChallengeID:      testChallengeID,
		Challenge:        challenge,
		CredentialID:     base64.RawURLEncoding.EncodeToString([]byte("credential-id-001")),
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeForbidden, apperrors.FromError(err).Code)
	assert.Empty(t, sessionID)
}

func TestCleanupExpiredWebAuthnChallenges_UsesClockMinusRetention(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := webauthnmock.NewMockRepository(ctrl)
	svc := newTestService(loginTestUnitOfWork{webAuthnRepo: repo})

	// 截止点必须是「注入时钟 - 保留期」，而不是 time.Now()：清理边界与挑战签发用同一只钟。
	repo.EXPECT().
		DeleteExpiredChallenges(gomock.Any(), svc.Clock.Now().Add(-WebAuthnChallengeRetention)).
		Return(int64(7), nil)

	deleted, err := svc.CleanupExpiredWebAuthnChallenges(context.Background(), WebAuthnChallengeRetention)
	require.NoError(t, err)
	assert.Equal(t, int64(7), deleted)
}

func TestWebAuthnAuthenticationFinish_RejectsForeignOrigin(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	// 被诱导到攻击者站点完成的断言：type / challenge / 签名都合法，只有 origin 不是我们的。
	clientData, authData, signature, coseKey := signWebAuthnAssertionWithOrigin(
		t, org.Slug, challenge, "https://evil.example.com", 1)
	credential := activeCredential(user.ID, coseKey)

	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	credentialRepo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	// UpdateSignCount 不得被调用：origin 校验失败必须发生在任何写入之前。

	sessionRepo := &loginTestSessionRepository{}
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     sessionRepo,
		webAuthnRepo: credentialRepo,
	})

	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  org.Slug,
		Username:          user.Username,
		ChallengeID:       testChallengeID,
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(credential.CredentialID),
		Signature:         signature,
		AuthenticatorData: authData,
		ClientDataJSON:    clientData,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	assert.Contains(t, apperrors.FromError(err).Message, "origin")
	assert.Empty(t, sessionID)
	assert.Empty(t, sessionRepo.sessions)
}

func TestWebAuthnAuthenticationFinish_FailsClosedWithoutOriginConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     &loginTestSessionRepository{},
		webAuthnRepo: credentialRepo,
	})
	// 部署既没有 CORS 白名单也没有 ISSUER_URL：来源集合为空 → 配置错误必须显式报出，
	// 且要在消费挑战之前失败（这里没有任何仓储期望，挑战被消费即测试失败）。
	svc.WebAuthnAllowedOrigins = nil
	svc.IssuerURL = ""

	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug: org.Slug,
		Username:         user.Username,
		ChallengeID:      testChallengeID,
		Challenge:        "server-issued-challenge",
		CredentialID:     base64.RawURLEncoding.EncodeToString([]byte("credential-id-001")),
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	assert.Contains(t, apperrors.FromError(err).Message, "CORS_ALLOWED_ORIGINS")
	assert.Empty(t, sessionID)
}

func TestWebAuthnAuthenticationFinish_RevokesCredentialOnSignCountRegression(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	// 存量计数 5，断言只报 3：按 FIDO2 口径视为克隆嫌疑（AC-AUTH-008 后半段）。
	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, org.Slug, challenge, 3)
	credential := activeCredential(user.ID, coseKey)
	credential.SignCount = 5

	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	credentialRepo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	// 关键断言一：凭证被置为 revoked（撤销后连真私钥也过不了 S3 守卫）。
	credentialRepo.EXPECT().
		UpdateStatus(gomock.Any(), credential.ID, webauthndomain.CredentialStatusRevoked).
		Return(nil)
	// 关键断言二：计数不得被写回（否则下次同一回退会被当成正常递增）。

	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     &loginTestSessionRepository{},
		webAuthnRepo: credentialRepo,
	})

	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  org.Slug,
		Username:          user.Username,
		ChallengeID:       testChallengeID,
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(credential.CredentialID),
		Signature:         signature,
		AuthenticatorData: authData,
		ClientDataJSON:    clientData,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	assert.Contains(t, apperrors.FromError(err).Message, "revoked")
	assert.Empty(t, sessionID)
}

func TestWebAuthnAuthenticationFinish_RequiresUserVerificationWhenPolicyEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	// UP-only 断言（0x01）：认证器只证明「有人碰了它」，没有 PIN/生物识别。
	clientData, authData, signature, coseKey := signWebAuthnAssertion(t, org.Slug, challenge, 1)
	credential := activeCredential(user.ID, coseKey)

	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	credentialRepo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	// UpdateSignCount 不得被调用：策略拒绝必须发生在任何写入之前。

	sessionRepo := &loginTestSessionRepository{}
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     sessionRepo,
		webAuthnRepo: credentialRepo,
	})
	svc.WebAuthnRequireUV = true

	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  org.Slug,
		Username:          user.Username,
		ChallengeID:       testChallengeID,
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(credential.CredentialID),
		Signature:         signature,
		AuthenticatorData: authData,
		ClientDataJSON:    clientData,
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
	assert.Contains(t, apperrors.FromError(err).Message, "user verification")
	assert.Empty(t, sessionID)
	assert.Empty(t, sessionRepo.sessions)
}

func TestWebAuthnAuthenticationFinish_AcceptsUserVerifiedAssertionUnderPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	org := newTestOrg()
	user := newTestUser()
	challenge := "server-issued-challenge"

	// UP+UV（0x05）：本人验证通过，策略满足。
	clientData, authData, signature, coseKey := signWebAuthnAssertionFull(t, org.Slug, challenge, "https://login.example.com", 0x05, 1)
	credential := activeCredential(user.ID, coseKey)

	credentialRepo := webauthnmock.NewMockRepository(ctrl)
	stubIssuedChallenge(t, credentialRepo, webauthndomain.CeremonyTypeAuthentication, challenge, user.ID, org.Slug)
	credentialRepo.EXPECT().GetByCredentialID(gomock.Any(), credential.CredentialID).Return(credential, nil)
	credentialRepo.EXPECT().UpdateSignCount(gomock.Any(), credential.ID, int64(1)).Return(nil)

	sessionRepo := &loginTestSessionRepository{}
	svc := newTestService(loginTestUnitOfWork{
		orgRepo:      newQRTestOrgRepository(org),
		appRepo:      &loginTestAppRepository{apps: map[string]*applications.Application{"default-console": defaultConsoleApp(org.ID, org.Slug)}},
		userRepo:     newQRTestUserRepository(user),
		sessRepo:     sessionRepo,
		webAuthnRepo: credentialRepo,
	})
	svc.WebAuthnRequireUV = true

	_, sessionID, err := svc.WebAuthnAuthenticationFinish(context.Background(), WebAuthnAuthenticationFinishCommand{
		OrganizationSlug:  org.Slug,
		Username:          user.Username,
		ChallengeID:       testChallengeID,
		Challenge:         challenge,
		CredentialID:      base64.RawURLEncoding.EncodeToString(credential.CredentialID),
		Signature:         signature,
		AuthenticatorData: authData,
		ClientDataJSON:    clientData,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, sessionID)
	assert.Len(t, sessionRepo.sessions, 1)
}
