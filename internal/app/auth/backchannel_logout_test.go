package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	appmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	sessionmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	tokenmock "github.com/guliangiam/guliangiam/internal/domain/tokens/mock"
	infrajwt "github.com/guliangiam/guliangiam/internal/infra/jwt"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/guliangiam/guliangiam/internal/domain/auth/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ═══════════════════════════════════════════════════════════════════════
// signLogoutTokenWithJWKS 0% → 100%（3 路径：nil / 成功 / 签名失败）
// ═══════════════════════════════════════════════════════════════════════

func TestSignLogoutTokenWithJWKS_NilManager(t *testing.T) {
	token, err := signLogoutTokenWithJWKS(map[string]any{"sub": "user-1"}, nil, "RS256")
	assert.Empty(t, token)
	require.Error(t, err)

	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeInternal, target.Code)
}

func TestSignLogoutTokenWithJWKS_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jwks := mock.NewMockJWKSProvider(ctrl)
	jwks.EXPECT().
		SignTokenWithAlgorithm(gomock.Any(), "logout+jwt", "SM2-SM3").
		DoAndReturn(func(claims map[string]any, tokenType, algorithm string) (string, error) {
			assert.Equal(t, "logout+jwt", tokenType)
			assert.Equal(t, "SM2-SM3", algorithm)
			assert.Equal(t, "user-1", claims["sub"])
			return "signed-jwt-token", nil
		})

	token, err := signLogoutTokenWithJWKS(map[string]any{"sub": "user-1"}, jwks, "SM2-SM3")
	require.NoError(t, err)
	assert.Equal(t, "signed-jwt-token", token)
}

func TestSignLogoutTokenWithJWKS_SignError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jwks := mock.NewMockJWKSProvider(ctrl)
	jwks.EXPECT().
		SignTokenWithAlgorithm(gomock.Any(), "logout+jwt", "RS256").
		Return("", errors.New("key not available"))

	token, err := signLogoutTokenWithJWKS(map[string]any{"sub": "user-1"}, jwks, "RS256")
	assert.Empty(t, token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key not available")
}

// ═══════════════════════════════════════════════════════════════════════
// logout token 算法判决 — 跟随应用 token_signing_method
// ═══════════════════════════════════════════════════════════════════════

func TestLogoutTokenSigningAlgorithm(t *testing.T) {
	cases := []struct {
		name   string
		method string
		want   string
	}{
		{"sm2-sm3", applicationdomain.TokenSigningMethodSM2SM3, "SM2-SM3"},
		{"rs256", applicationdomain.TokenSigningMethodRS256, "RS256"},
		{"empty falls back to rs256", "", "RS256"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			algorithm, err := logoutTokenSigningAlgorithm(testCase.method)
			require.NoError(t, err)
			assert.Equal(t, testCase.want, algorithm)
		})
	}
}

func TestLogoutTokenSigningAlgorithm_RejectsOutOfEnum(t *testing.T) {
	for _, method := range []string{"HS256", "ES256", "SM2-SM3-TEST"} {
		t.Run(method, func(t *testing.T) {
			algorithm, err := logoutTokenSigningAlgorithm(method)
			assert.Empty(t, algorithm)

			var target *apperrors.Error
			require.ErrorAs(t, err, &target)
			assert.Equal(t, apperrors.CodeInvalidArgument, target.Code)
		})
	}
}

// TestSignLogoutTokenWithJWKS_AlgorithmAndKid delegates to the real adapter: a
// stub that echoed the parameters would not catch an RS256 signature wearing an
// SM2-SM3 label, so the JOSE header and the kid prefix are asserted here.
func TestSignLogoutTokenWithJWKS_AlgorithmAndKid(t *testing.T) {
	provider := infrajwt.NewJWKSProviderAdapter(platformjwks.NewManager(2048, nil))

	cases := []struct {
		name          string
		method        string
		wantAlgorithm string
		wantKidPrefix string
	}{
		{"sm2-sm3", applicationdomain.TokenSigningMethodSM2SM3, "SM2-SM3", "gm_"},
		{"rs256", applicationdomain.TokenSigningMethodRS256, "RS256", "rsa_"},
		{"empty falls back to rs256", "", "RS256", "rsa_"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			algorithm, err := logoutTokenSigningAlgorithm(testCase.method)
			require.NoError(t, err)

			token, err := signLogoutTokenWithJWKS(
				map[string]any{"sub": "user-1"}, provider, algorithm)
			require.NoError(t, err)

			header := decodeTokenHeader(t, token)
			assert.Equal(t, testCase.wantAlgorithm, header["alg"])
			assert.True(t, strings.HasPrefix(header["kid"], testCase.wantKidPrefix),
				"kid = %q, want %q prefix", header["kid"], testCase.wantKidPrefix)
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════
// SendBackchannelLogout 0% — 参数校验早返回（不触发事务）
// ═══════════════════════════════════════════════════════════════════════

func TestSendBackchannelLogout_UserIDRequired(t *testing.T) {
	svc := &Service{}
	result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
		UserID:   "",
		ClientID: "client-abc",
	})
	assert.Nil(t, result)
	require.Error(t, err)

	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeInvalidArgument, target.Code)
}

func TestSendBackchannelLogout_ClientIDRequired(t *testing.T) {
	svc := &Service{}
	result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
		UserID:   "user-1",
		ClientID: "",
	})
	assert.Nil(t, result)
	require.Error(t, err)

	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeInvalidArgument, target.Code)
}

func TestSendBackchannelLogout_RequiresAuthenticatedActor(t *testing.T) {
	svc := &Service{}
	result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
		UserID:         "user-1",
		OrganizationID: "org-1",
		ClientID:       "client-abc",
	})
	assert.Nil(t, result)
	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeUnauthorized, target.Code)
}

func TestSendBackchannelLogout_RejectsCrossUserRevocation(t *testing.T) {
	svc := &Service{}
	result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
		UserID:         "user-1",
		OrganizationID: "org-1",
		ActorID:        "user-2",
		ActorType:      "user",
		ClientID:       "client-abc",
	})
	assert.Nil(t, result)
	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeForbidden, target.Code)
}

func TestSendBackchannelLogout_RejectsCrossOrganizationApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockAuthUoW(ctrl)
	fullTx := uowmock.NewMockUnitOfWork(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)

	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, fullTx)
		})
	fullTx.EXPECT().Applications().Return(appRepo)
	appRepo.EXPECT().GetByClientID(gomock.Any(), "client-abc").
		Return(&applicationdomain.Application{ID: "app-1", OrganizationID: "org-2", ClientID: "client-abc"}, nil)

	svc := &Service{UOW: uow}
	result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
		UserID:         "user-1",
		OrganizationID: "org-1",
		ActorID:        "user-1",
		ActorType:      "user",
		ClientID:       "client-abc",
	})
	assert.Nil(t, result)
	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeForbidden, target.Code)
}

// ═══════════════════════════════════════════════════════════════════════
// SendBackchannelLogout — 事务内 application 找不到
// ═══════════════════════════════════════════════════════════════════════

func TestSendBackchannelLogout_ApplicationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	fullTx := uowmock.NewMockUnitOfWork(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)

	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, fullTx)
		})
	fullTx.EXPECT().Applications().Return(appRepo)
	appRepo.EXPECT().GetByClientID(gomock.Any(), "client-abc").
		Return((*applicationdomain.Application)(nil), errors.New("not found"))

	svc := &Service{UOW: uow}
	result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
		UserID:         "user-1",
		OrganizationID: "org-1",
		ActorID:        "user-1",
		ActorType:      "user",
		ClientID:       "client-abc",
	})
	assert.Nil(t, result)
	require.Error(t, err)

	// callback 返回 NewNotFound → 事务外被 WrapInternal
	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeInternal, target.Code)
}

// ═══════════════════════════════════════════════════════════════════════
// SendBackchannelLogout — 完整成功路径（BackchannelLogoutUri 空 → generated）
// ═══════════════════════════════════════════════════════════════════════

func TestSendBackchannelLogout_SuccessGenerated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	fullTx := uowmock.NewMockUnitOfWork(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)
	tokenRepo := tokenmock.NewMockRepository(ctrl)
	jwks := mock.NewMockJWKSProvider(ctrl)

	application := &applicationdomain.Application{
		ID:                   "app-1",
		OrganizationID:       "org-1",
		ClientID:             "client-abc",
		BackchannelLogoutUri: "", // 空 → "generated" 状态
	}

	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, fullTx)
		})
	fullTx.EXPECT().Applications().Return(appRepo)
	appRepo.EXPECT().GetByClientID(gomock.Any(), "client-abc").
		Return(application, nil)

	fullTx.EXPECT().Tokens().Return(tokenRepo)
	tokenRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, query any, revokedAt time.Time) (int64, error) {
			return 5, nil
		})

	// JWKS signing — claims 包含 iss/aud/sub/events；空 token_signing_method 回落 RS256
	jwks.EXPECT().
		SignTokenWithAlgorithm(gomock.Any(), "logout+jwt", "RS256").
		DoAndReturn(func(claims map[string]any, tokenType, algorithm string) (string, error) {
			assert.Equal(t, "client-abc", claims["aud"])
			assert.Equal(t, "user-1", claims["sub"])
			return "signed-jwt-token", nil
		})

	svc := &Service{
		UOW:           uow,
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		JWKSManager:   jwks,
		IssuerURL:     "https://issuer.example",
	}

	result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
		UserID:         "user-1",
		OrganizationID: "org-1",
		ActorID:        "platform-admin-1",
		ActorType:      "platform_admin",
		ClientID:       "client-abc",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "signed-jwt-token", result.LogoutToken)
	assert.Equal(t, "generated", result.Status) // BackchannelLogoutUri 空
	assert.Empty(t, result.Error)
}

// TestSendBackchannelLogout_SigningAlgorithmFollowsApplication wires the real
// signer into the use case, so the application's token_signing_method must
// survive the command → repository → signing path and show up in the JOSE
// header of the delivered logout token.
func TestSendBackchannelLogout_SigningAlgorithmFollowsApplication(t *testing.T) {
	provider := infrajwt.NewJWKSProviderAdapter(platformjwks.NewManager(2048, nil))

	cases := []struct {
		name          string
		method        string
		wantAlgorithm string
		wantKidPrefix string
	}{
		{"sm2-sm3", applicationdomain.TokenSigningMethodSM2SM3, "SM2-SM3", "gm_"},
		{"rs256", applicationdomain.TokenSigningMethodRS256, "RS256", "rsa_"},
		{"empty falls back to rs256", "", "RS256", "rsa_"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			uow := uowmock.NewMockAuthUoW(ctrl)
			fullTx := uowmock.NewMockUnitOfWork(ctrl)
			appRepo := appmock.NewMockRepository(ctrl)
			tokenRepo := tokenmock.NewMockRepository(ctrl)

			application := &applicationdomain.Application{
				ID:                 "app-1",
				OrganizationID:     "org-1",
				ClientID:           "client-abc",
				TokenSigningMethod: testCase.method,
			}

			uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
					return fn(ctx, fullTx)
				})
			fullTx.EXPECT().Applications().Return(appRepo)
			appRepo.EXPECT().GetByClientID(gomock.Any(), "client-abc").Return(application, nil)
			fullTx.EXPECT().Tokens().Return(tokenRepo)
			tokenRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(5), nil)

			svc := &Service{
				UOW:           uow,
				Clock:         clock.SystemClock{},
				UUIDGenerator: platformuuid.V7Generator{},
				JWKSManager:   provider,
				IssuerURL:     "https://issuer.example",
			}

			result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
				UserID:         "user-1",
				OrganizationID: "org-1",
				ActorID:        "user-1",
				ActorType:      "user",
				ClientID:       "client-abc",
			})
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "generated", result.Status)

			header := decodeTokenHeader(t, result.LogoutToken)
			assert.Equal(t, testCase.wantAlgorithm, header["alg"])
			assert.True(t, strings.HasPrefix(header["kid"], testCase.wantKidPrefix),
				"kid = %q, want %q prefix", header["kid"], testCase.wantKidPrefix)
		})
	}
}

// TestSendBackchannelLogout_RejectsUnknownTokenSigningMethod pins requirement
// 3: an application value outside the closed enum is an InvalidArgument, not a
// silent RS256 downgrade. The mock signer has no expectation, so any signing
// attempt also fails the test.
func TestSendBackchannelLogout_RejectsUnknownTokenSigningMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	fullTx := uowmock.NewMockUnitOfWork(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)
	jwks := mock.NewMockJWKSProvider(ctrl)

	application := &applicationdomain.Application{
		ID:                 "app-1",
		OrganizationID:     "org-1",
		ClientID:           "client-abc",
		TokenSigningMethod: "HS256",
	}

	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, fullTx)
		})
	fullTx.EXPECT().Applications().Return(appRepo)
	appRepo.EXPECT().GetByClientID(gomock.Any(), "client-abc").Return(application, nil)

	svc := &Service{
		UOW:           uow,
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		JWKSManager:   jwks,
	}

	result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
		UserID:         "user-1",
		OrganizationID: "org-1",
		ActorID:        "user-1",
		ActorType:      "user",
		ClientID:       "client-abc",
	})
	assert.Nil(t, result)
	require.Error(t, err)

	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeInvalidArgument, target.Code)
}

// ═══════════════════════════════════════════════════════════════════════
// postBackchannelLogoutToken 0% — httptest.NewServer 零 mock
// ═══════════════════════════════════════════════════════════════════════

func TestPostBackchannelLogoutToken_Success(t *testing.T) {
	// SSRF client blocks loopback → override with default client
	origClient := backchannelHTTPClient
	backchannelHTTPClient = http.DefaultClient
	t.Cleanup(func() { backchannelHTTPClient = origClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "logout-token-xyz", r.FormValue("logout_token"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := postBackchannelLogoutToken(context.Background(), server.URL, "logout-token-xyz")
	assert.NoError(t, err)
}

func TestPostBackchannelLogoutToken_Non200Response(t *testing.T) {
	origClient := backchannelHTTPClient
	backchannelHTTPClient = http.DefaultClient
	t.Cleanup(func() { backchannelHTTPClient = origClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("rp internal error"))
	}))
	defer server.Close()

	err := postBackchannelLogoutToken(context.Background(), server.URL, "logout-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "rp internal error")
}

func TestPostBackchannelLogoutToken_RequestCreationError(t *testing.T) {
	// 非法 URL — http.NewRequestWithContext 返回 error
	err := postBackchannelLogoutToken(context.Background(), "://bad-url", "token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create backchannel logout request")
}

// ═══════════════════════════════════════════════════════════════════════
// SendBackchannelLogout — 带 BackchannelLogoutUri → delivered / delivered_with_error
// ═══════════════════════════════════════════════════════════════════════

func TestSendBackchannelLogout_Delivered(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// SSRF client blocks loopback → override with default client
	origClient := backchannelHTTPClient
	backchannelHTTPClient = http.DefaultClient
	t.Cleanup(func() { backchannelHTTPClient = origClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	uow := uowmock.NewMockAuthUoW(ctrl)
	fullTx := uowmock.NewMockUnitOfWork(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)
	tokenRepo := tokenmock.NewMockRepository(ctrl)
	jwks := mock.NewMockJWKSProvider(ctrl)

	application := &applicationdomain.Application{
		ID:                   "app-1",
		OrganizationID:       "org-1",
		ClientID:             "client-abc",
		BackchannelLogoutUri: server.URL, // 非空 → HTTP POST
	}

	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, fullTx)
		})
	fullTx.EXPECT().Applications().Return(appRepo)
	appRepo.EXPECT().GetByClientID(gomock.Any(), "client-abc").Return(application, nil)
	fullTx.EXPECT().Tokens().Return(tokenRepo)
	tokenRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(5), nil)
	jwks.EXPECT().SignTokenWithAlgorithm(gomock.Any(), "logout+jwt", "RS256").Return("signed-token", nil)

	svc := &Service{
		UOW:           uow,
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		JWKSManager:   jwks,
		IssuerURL:     "https://issuer.example",
	}

	result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
		UserID:         "user-1",
		OrganizationID: "org-1",
		ActorID:        "user-1",
		ActorType:      "user",
		ClientID:       "client-abc",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "signed-token", result.LogoutToken)
	assert.Equal(t, "delivered", result.Status) // httptest 返回 200
	assert.Empty(t, result.Error)
}

func TestSendBackchannelLogout_DeliveredWithError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// SSRF client blocks loopback → override with default client
	origClient := backchannelHTTPClient
	backchannelHTTPClient = http.DefaultClient
	t.Cleanup(func() { backchannelHTTPClient = origClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("RP error"))
	}))
	defer server.Close()

	uow := uowmock.NewMockAuthUoW(ctrl)
	fullTx := uowmock.NewMockUnitOfWork(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)
	tokenRepo := tokenmock.NewMockRepository(ctrl)
	jwks := mock.NewMockJWKSProvider(ctrl)

	application := &applicationdomain.Application{
		ID:                   "app-1",
		OrganizationID:       "org-1",
		ClientID:             "client-abc",
		BackchannelLogoutUri: server.URL,
	}

	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, fullTx)
		})
	fullTx.EXPECT().Applications().Return(appRepo)
	appRepo.EXPECT().GetByClientID(gomock.Any(), "client-abc").Return(application, nil)
	fullTx.EXPECT().Tokens().Return(tokenRepo)
	tokenRepo.EXPECT().RevokeByQuery(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(5), nil)
	jwks.EXPECT().SignTokenWithAlgorithm(gomock.Any(), "logout+jwt", "RS256").Return("signed-token", nil)

	svc := &Service{
		UOW:           uow,
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		JWKSManager:   jwks,
		IssuerURL:     "https://issuer.example",
	}

	result, err := svc.SendBackchannelLogout(context.Background(), SendBackchannelLogoutCommand{
		UserID:         "user-1",
		OrganizationID: "org-1",
		ActorID:        "user-1",
		ActorType:      "user",
		ClientID:       "client-abc",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "delivered_with_error", result.Status) // httptest 返回 500
	assert.Contains(t, result.Error, "500")
}

// ═══════════════════════════════════════════════════════════════════════
// sendBackchannelLogoutNotifications 0% — fire-and-forget 路径
// ═══════════════════════════════════════════════════════════════════════

func TestSendBackchannelLogoutNotifications_SessionsListError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	sessionRepo := sessionmock.NewMockRepository(ctrl)

	uow.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*sessiondomain.Session(nil), int64(0), errors.New("db down"))

	svc := &Service{UOW: uow}
	// fire-and-forget，直接调用，不 panic
	svc.sendBackchannelLogoutNotifications(context.Background(), "user-1", "https://issuer")
}

func TestSendBackchannelLogoutNotifications_EmptySessions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	sessionRepo := sessionmock.NewMockRepository(ctrl)

	uow.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*sessiondomain.Session{}, int64(0), nil)

	svc := &Service{UOW: uow}
	svc.sendBackchannelLogoutNotifications(context.Background(), "user-1", "https://issuer")
}

func TestSendBackchannelLogoutNotifications_SkipsEmptyApplicationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	sessionRepo := sessionmock.NewMockRepository(ctrl)

	uow.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*sessiondomain.Session{{ApplicationID: ""}}, int64(1), nil)

	svc := &Service{UOW: uow}
	svc.sendBackchannelLogoutNotifications(context.Background(), "user-1", "https://issuer")
}

func TestSendBackchannelLogoutNotifications_SkipsEmptyBackchannelURI(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	sessionRepo := sessionmock.NewMockRepository(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)

	uow.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*sessiondomain.Session{{ApplicationID: "app-1", Sid: "sid-1"}}, int64(1), nil)

	uow.EXPECT().Applications().Return(appRepo)
	appRepo.EXPECT().GetByID(gomock.Any(), "app-1").
		Return(&applicationdomain.Application{BackchannelLogoutUri: ""}, nil)

	svc := &Service{UOW: uow}
	svc.sendBackchannelLogoutNotifications(context.Background(), "user-1", "https://issuer")
}

func TestSendBackchannelLogoutNotifications_FanOutSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// SSRF client blocks loopback → override
	origClient := backchannelHTTPClient
	backchannelHTTPClient = http.DefaultClient
	t.Cleanup(func() { backchannelHTTPClient = origClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	uow := uowmock.NewMockAuthUoW(ctrl)
	sessionRepo := sessionmock.NewMockRepository(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)
	jwks := mock.NewMockJWKSProvider(ctrl)

	uow.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*sessiondomain.Session{
			{ApplicationID: "app-1", Sid: "sid-1"},
			{ApplicationID: "app-1", Sid: "sid-1-dup"}, // 同一 application 应被折叠
			{ApplicationID: "app-2", Sid: "sid-2"},
		}, int64(3), nil)

	// app-1 和 app-2 各一次 GetByID（折叠后）
	uow.EXPECT().Applications().Return(appRepo).Times(2)
	appRepo.EXPECT().GetByID(gomock.Any(), "app-1").
		Return(&applicationdomain.Application{
			ID:                   "app-1",
			ClientID:             "client-1",
			BackchannelLogoutUri: server.URL,
		}, nil)
	appRepo.EXPECT().GetByID(gomock.Any(), "app-2").
		Return(&applicationdomain.Application{
			ID:                   "app-2",
			ClientID:             "client-2",
			BackchannelLogoutUri: server.URL,
			TokenSigningMethod:   applicationdomain.TokenSigningMethodSM2SM3,
		}, nil)

	// 每个 application 按自身 token_signing_method 签名：app-1 空值 → RS256，app-2 → SM2-SM3
	jwks.EXPECT().SignTokenWithAlgorithm(gomock.Any(), "logout+jwt", "RS256").
		Return("signed-client-1", nil)
	jwks.EXPECT().SignTokenWithAlgorithm(gomock.Any(), "logout+jwt", "SM2-SM3").
		Return("signed-client-2", nil)

	svc := &Service{
		UOW:           uow,
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		JWKSManager:   jwks,
	}
	svc.sendBackchannelLogoutNotifications(context.Background(), "user-1", "https://issuer")
}

// TestSendBackchannelLogoutNotifications_SkipsUnknownTokenSigningMethod pins the
// fan-out half of requirement 3: an application with an out-of-enum value is
// skipped, never signed with the RS256 default. The mock signer has no
// expectation, so a downgrade would fail the test.
func TestSendBackchannelLogoutNotifications_SkipsUnknownTokenSigningMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	sessionRepo := sessionmock.NewMockRepository(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)
	jwks := mock.NewMockJWKSProvider(ctrl)

	uow.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*sessiondomain.Session{{ApplicationID: "app-1", Sid: "sid-1"}}, int64(1), nil)
	uow.EXPECT().Applications().Return(appRepo)
	appRepo.EXPECT().GetByID(gomock.Any(), "app-1").
		Return(&applicationdomain.Application{
			ID:                   "app-1",
			ClientID:             "client-1",
			BackchannelLogoutUri: "https://rp.example/backchannel",
			TokenSigningMethod:   "ES256",
		}, nil)

	svc := &Service{
		UOW:           uow,
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		JWKSManager:   jwks,
	}
	svc.sendBackchannelLogoutNotifications(context.Background(), "user-1", "https://issuer")
}
