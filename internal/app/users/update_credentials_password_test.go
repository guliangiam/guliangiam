package users

import (
	"context"
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/cred"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TestUpdateUserCredentials_RejectsPasswordField 覆盖端点契约：PUT
// /users/{id}/credentials 携带 password 一律 422，口令唯一写路径是
// POST /users/{id}/set-password（users:password:reset）。
func TestUpdateUserCredentials_RejectsPasswordField(t *testing.T) {
	service := &Service{UOW: testUnitOfWork{}}

	_, err := service.UpdateUserCredentials(context.Background(), UpdateUserCredentialsCommand{
		OrganizationID: testOrganizationID,
		UserID:         testUserID,
		Password:       "Str0ng!Passw0rd",
		ActorID:        "operator",
	})
	requireCode(t, err, apperrors.CodeUnprocessableEntity)
	if err == nil {
		t.Fatal("password on credentials endpoint must be rejected")
	}
}

// TestUpdateUserCredentials_RotationGracePeriod 覆盖轮换宽限期语义：prev
// 过期时间以「本次轮换时间 + AccessSecretGracePeriod」计算，与被轮换 secret
// 自身的激活时间无关。回归缺陷：曾用旧 rotatedAt 推算过期时间，导致存活
// 超过宽限期的 secret 被轮换后 prev 立即失效，滚动更新窗口形同虚设。
func TestUpdateUserCredentials_RotationGracePeriod(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	oldSecret := "0123456789abcdef0123456789abcdef01234567"
	oldHash, err := cred.HashArgon2id(oldSecret)
	if err != nil {
		t.Fatalf("hash old secret: %v", err)
	}
	// 旧 secret 于一小时前激活——被轮换时已存活超过 30 分钟宽限期（常态）。
	rotatedAt := fixedNow.Add(-time.Hour)

	user := newTestUser()
	user.Credentials = userdomain.UserCredentials{
		UserID:                testUserID,
		AccessKey:             "oldaccesskey",
		AccessSecretHash:      oldHash,
		AccessSecretRotatedAt: &rotatedAt,
	}

	var persisted *userdomain.UserCredentials
	userRepo := &mockUserRepo{
		getByIDFn: func(_ context.Context, _ string) (*userdomain.User, error) {
			return user, nil
		},
		updateCredentialsFn: func(_ context.Context, credentials *userdomain.UserCredentials) error {
			c := *credentials
			persisted = &c
			return nil
		},
	}

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UOW: testUnitOfWork{
			users:    userRepo,
			sessions: &mockSessionRepo{},
			tokens:   &mockTokenRepo{},
			records:  &fakeRecordsRepo{},
		},
	}

	result, err := svc.UpdateUserCredentials(context.Background(), UpdateUserCredentialsCommand{
		OrganizationID:     testOrganizationID,
		UserID:             testUserID,
		RotateAccessSecret: true,
		ActorID:            "operator",
	})
	if err != nil {
		t.Fatalf("UpdateUserCredentials: %v", err)
	}
	if persisted == nil {
		t.Fatal("credentials not persisted")
	}

	wantExpiry := fixedNow.Add(userdomain.AccessSecretGracePeriod)
	if persisted.AccessSecretHashPrev != oldHash {
		t.Fatalf("prev hash = %q, want rotated-out old hash", persisted.AccessSecretHashPrev)
	}
	if persisted.AccessSecretPrevExpiresAt == nil || !persisted.AccessSecretPrevExpiresAt.Equal(wantExpiry) {
		t.Fatalf("prev expiry = %v, want %v (rotation time + grace period)",
			persisted.AccessSecretPrevExpiresAt, wantExpiry)
	}

	verifier := cred.GetCredManager(userdomain.PasswordAlgorithmArgon2ID)
	// 宽限期内：旧明文 secret 仍必须通过校验。
	if !persisted.VerifyAccessSecret(verifier, oldSecret, fixedNow.Add(time.Minute)) {
		t.Fatal("old secret must verify within the grace period")
	}
	// 宽限期后：旧明文 secret 必须 fail closed。
	if persisted.VerifyAccessSecret(verifier, oldSecret, fixedNow.Add(userdomain.AccessSecretGracePeriod+time.Minute)) {
		t.Fatal("old secret must fail after the grace period")
	}
	// 新明文 secret 立即可用。
	if !persisted.VerifyAccessSecret(verifier, result.RotatedAccessSecret, fixedNow) {
		t.Fatal("rotated secret must verify immediately")
	}
}

// TestUpdateUserCredentials_RotationMustNotRevokeSessions 是本次修复的核心
// 回归用例：Access Secret 轮换属于「机器端 API 凭证」变更，语义上不应该
// 撤销该用户的浏览器会话、刷新令牌或访问令牌——浏览器登录态与 API key
// 是两套独立体系。原 Bug：credentialsChanged 变量错误地绑定了
// RotateAccessSecret，导致每次轮换都触发 ForceLogoutUser 全链路。
func TestUpdateUserCredentials_RotationMustNotRevokeSessions(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	user := newTestUser()
	user.Credentials = userdomain.UserCredentials{
		UserID:           testUserID,
		AccessKey:        "oldaccesskey",
		AccessSecretHash: "oldhash",
	}

	userRepo := &mockUserRepo{
		getByIDFn: func(_ context.Context, _ string) (*userdomain.User, error) {
			return user, nil
		},
		updateCredentialsFn: func(_ context.Context, _ *userdomain.UserCredentials) error {
			return nil
		},
	}

	sessionRevoked := false
	sessionRepo := &mockSessionRepo{
		revokeByUserFn: func(_ context.Context, _ string, _ time.Time) (int64, error) {
			sessionRevoked = true
			return 0, nil
		},
	}

	tokenRevoked := false
	refreshRevoked := false
	tokenRepo := &mockTokenRepo{
		revokeByQueryFn: func(_ context.Context, _ tokendomain.RevokeQuery, _ time.Time) (int64, error) {
			tokenRevoked = true
			return 0, nil
		},
		revokeRefreshByUserFn: func(_ context.Context, _ string, _ time.Time) (int64, error) {
			refreshRevoked = true
			return 0, nil
		},
	}

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UOW: testUnitOfWork{
			users:    userRepo,
			sessions: sessionRepo,
			tokens:   tokenRepo,
			records:  &fakeRecordsRepo{},
		},
	}

	_, err := svc.UpdateUserCredentials(context.Background(), UpdateUserCredentialsCommand{
		OrganizationID:     testOrganizationID,
		UserID:             testUserID,
		RotateAccessSecret: true,
		ActorID:            "operator",
	})
	if err != nil {
		t.Fatalf("UpdateUserCredentials: %v", err)
	}

	if sessionRevoked {
		t.Fatal("Access Secret rotation must NOT revoke user sessions")
	}
	if tokenRevoked {
		t.Fatal("Access Secret rotation must NOT revoke user access tokens")
	}
	if refreshRevoked {
		t.Fatal("Access Secret rotation must NOT revoke user refresh tokens")
	}
}
