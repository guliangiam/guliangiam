package auth

import (
	"context"
	"errors"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	appdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	appmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// runTxSetup 构造 transaction 内的 mock 链：
// uowMock (AuthUoW) → WithinTransaction DoAndReturn → 调用 callback 传入 fullTx
// fullTx (完整 UnitOfWork) → Applications() → appRepo
func runTxSetup(ctrl *gomock.Controller, app *appdomain.Application, getErr error, appID string) *uowmock.MockAuthUoW {
	uowMock := uowmock.NewMockAuthUoW(ctrl)
	fullTx := uowmock.NewMockUnitOfWork(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)

	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, fullTx)
		})
	fullTx.EXPECT().Applications().Return(appRepo)
	appRepo.EXPECT().GetByClientID(gomock.Any(), appID).Return(app, getErr)

	return uowMock
}

// ═══════════════════════════════════════════════════════════════════════
// AuthenticateClient 测试
// ═══════════════════════════════════════════════════════════════════════

func TestAuthenticateClient_EmptyClientID_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}}

	result, err := svc.AuthenticateClient(context.Background(), AuthenticateClientCommand{ClientID: ""})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestAuthenticateClient_WhitespaceOnlyClientID_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}}

	result, err := svc.AuthenticateClient(context.Background(), AuthenticateClientCommand{ClientID: "   "})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestAuthenticateClient_GetByClientIDError_ReturnsUnauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := runTxSetup(ctrl, nil, errors.New("not found"), "client-123")
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	result, err := svc.AuthenticateClient(context.Background(), AuthenticateClientCommand{ClientID: "client-123"})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestAuthenticateClient_PublicClientWithCredentials_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	app := &appdomain.Application{
		ID: "app-1", ClientID: "public-client", TokenEndpointAuthMethod: "none",
	}
	uowMock := runTxSetup(ctrl, app, nil, "public-client")
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	result, err := svc.AuthenticateClient(context.Background(), AuthenticateClientCommand{
		ClientID: "public-client", ClientSecret: "should-not-be-here",
	})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestAuthenticateClient_PublicClientNoCredentials_Succeeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	app := &appdomain.Application{
		ID: "app-1", ClientID: "public-client", TokenEndpointAuthMethod: "none",
	}
	uowMock := runTxSetup(ctrl, app, nil, "public-client")
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	result, err := svc.AuthenticateClient(context.Background(), AuthenticateClientCommand{ClientID: "public-client"})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Authenticated)
	assert.Equal(t, "public-client", result.ClientID)
	assert.Equal(t, "app-1", result.ApplicationID)
}

func TestAuthenticateClient_SecretBasicMissingSecret_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	app := &appdomain.Application{
		ID: "app-1", ClientID: "secret-client",
		TokenEndpointAuthMethod: "client_secret_basic", ClientSecretHash: "somehash",
	}
	uowMock := runTxSetup(ctrl, app, nil, "secret-client")
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	result, err := svc.AuthenticateClient(context.Background(), AuthenticateClientCommand{ClientID: "secret-client"})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestAuthenticateClient_UnknownAssertionType_Rejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	app := &appdomain.Application{
		ID: "app-1", ClientID: "weird-client", TokenEndpointAuthMethod: "weird_method",
	}
	uowMock := runTxSetup(ctrl, app, nil, "weird-client")
	svc := Service{Clock: clock.SystemClock{}, UUIDGenerator: platformuuid.V7Generator{}, UOW: uowMock}

	result, err := svc.AuthenticateClient(context.Background(), AuthenticateClientCommand{
		ClientID: "weird-client", ClientAssertion: "some.jwt.token",
		ClientAssertionType: "urn:ietf:params:oauth:client-assertion-type:x509cert",
	})
	require.Error(t, err)
	assert.Nil(t, result)
}
