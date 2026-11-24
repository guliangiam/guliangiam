package auth

import (
	"context"
	"testing"
	"time"

	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	deviceauthdomain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	deviceauthmock "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newDeviceAuthTestSvc(uow *uowmock.MockAuthUoW) Service {
	return Service{
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		UOW:           uow,
	}
}

// ═══════════════════════════════════════════════════════════════════════
// DeviceAuthorize
// ═══════════════════════════════════════════════════════════════════════

func TestDeviceAuthorize_ClientIDRequired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := newDeviceAuthTestSvc(uowmock.NewMockAuthUoW(ctrl))
	_, err := svc.DeviceAuthorize(context.Background(), DeviceAuthorizeCommand{})
	require.Error(t, err)
}

func TestDeviceAuthorize_Success_CreatesCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	dacRepo := deviceauthmock.NewMockRepository(ctrl)
	uow.EXPECT().DeviceAuthCodes().Return(dacRepo)
	dacRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, entry *deviceauthdomain.DeviceAuthCode) error {
			require.NotEmpty(t, entry.ID)
			require.NotEmpty(t, entry.DeviceCode)
			require.NotEmpty(t, entry.UserCode)
			assert.Equal(t, "client-x", entry.ClientID)
			return nil
		})

	svc := newDeviceAuthTestSvc(uow)
	result, err := svc.DeviceAuthorize(context.Background(), DeviceAuthorizeCommand{
		ClientID:  "client-x",
		Scope:     "openid profile",
		IssuerURL: "https://idp.example.com",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.DeviceCode)
	assert.NotEmpty(t, result.UserCode)
	assert.NotEmpty(t, result.VerificationURI)
	assert.Contains(t, result.VerificationURIComplete, result.UserCode)
	assert.Equal(t, int64(deviceCodeTTL/time.Second), result.ExpiresIn)
	assert.Equal(t, int64(deviceCodePollInterval/time.Second), result.Interval)
}

func TestDeviceAuthorize_CreateError_WrapsInternal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	dacRepo := deviceauthmock.NewMockRepository(ctrl)
	uow.EXPECT().DeviceAuthCodes().Return(dacRepo)
	dacRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(assert.AnError)

	svc := newDeviceAuthTestSvc(uow)
	_, err := svc.DeviceAuthorize(context.Background(), DeviceAuthorizeCommand{
		ClientID: "client-x",
	})
	require.Error(t, err)
	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeInternal, target.Code)
}

// ═══════════════════════════════════════════════════════════════════════
// DeviceVerify
// ═══════════════════════════════════════════════════════════════════════

func TestDeviceVerify_UserCodeRequired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := newDeviceAuthTestSvc(uowmock.NewMockAuthUoW(ctrl))
	err := svc.DeviceVerify(context.Background(), DeviceVerifyCommand{})
	require.Error(t, err)
}

func TestDeviceVerify_Success_UpdatesVerification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now().UTC()
	uow := uowmock.NewMockAuthUoW(ctrl)
	dacRepo := deviceauthmock.NewMockRepository(ctrl)
	uow.EXPECT().DeviceAuthCodes().Return(dacRepo).Times(2)

	dacRepo.EXPECT().GetByUserCode(gomock.Any(), "ABCD-EFGH").Return(
		&deviceauthdomain.DeviceAuthCode{
			ID:        "code-1",
			UserCode:  "ABCD-EFGH",
			Verified:  false,
			ExpiresAt: now.Add(5 * time.Minute),
			CreatedAt: now,
			UpdatedAt: now,
			Revision:  1,
		}, nil,
	)
	dacRepo.EXPECT().UpdateVerification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, entry *deviceauthdomain.DeviceAuthCode) error {
			assert.True(t, entry.Verified)
			assert.Equal(t, "user-1", entry.UserID)
			assert.Equal(t, "org-1", entry.OrganizationID)
			return nil
		})

	svc := newDeviceAuthTestSvc(uow)
	err := svc.DeviceVerify(context.Background(), DeviceVerifyCommand{
		UserCode:       "ABCD-EFGH",
		UserID:         "user-1",
		Username:       "alice",
		OrganizationID: "org-1",
	})
	require.NoError(t, err)
}

func TestDeviceVerify_Expired_ReturnsInvalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	dacRepo := deviceauthmock.NewMockRepository(ctrl)
	uow.EXPECT().DeviceAuthCodes().Return(dacRepo)

	dacRepo.EXPECT().GetByUserCode(gomock.Any(), "ABCD-EFGH").Return(
		&deviceauthdomain.DeviceAuthCode{
			ID:        "code-1",
			UserCode:  "ABCD-EFGH",
			Verified:  false,
			ExpiresAt: time.Now().UTC().Add(-1 * time.Minute), // already expired
			Revision:  1,
		}, nil,
	)

	svc := newDeviceAuthTestSvc(uow)
	err := svc.DeviceVerify(context.Background(), DeviceVerifyCommand{
		UserCode: "ABCD-EFGH",
	})
	require.Error(t, err)
}

func TestDeviceVerify_AlreadyVerified_ReturnsConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	dacRepo := deviceauthmock.NewMockRepository(ctrl)
	uow.EXPECT().DeviceAuthCodes().Return(dacRepo)

	dacRepo.EXPECT().GetByUserCode(gomock.Any(), "ABCD-EFGH").Return(
		&deviceauthdomain.DeviceAuthCode{
			ID:        "code-1",
			UserCode:  "ABCD-EFGH",
			Verified:  true, // already verified
			ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
			Revision:  1,
		}, nil,
	)

	svc := newDeviceAuthTestSvc(uow)
	err := svc.DeviceVerify(context.Background(), DeviceVerifyCommand{
		UserCode: "ABCD-EFGH",
	})
	require.Error(t, err)
	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeConflict, target.Code)
}

// ═══════════════════════════════════════════════════════════════════════
// CancelDeviceAuthorization
// ═══════════════════════════════════════════════════════════════════════

func TestCancelDeviceAuthorization_DeviceCodeRequired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := newDeviceAuthTestSvc(uowmock.NewMockAuthUoW(ctrl))
	err := svc.CancelDeviceAuthorization(context.Background(), DeviceCancelCommand{})
	require.Error(t, err)
}

func TestCancelDeviceAuthorization_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	dacRepo := deviceauthmock.NewMockRepository(ctrl)
	uow.EXPECT().DeviceAuthCodes().Return(dacRepo).Times(2)

	dacRepo.EXPECT().GetByDeviceCode(gomock.Any(), "device-code-1").Return(
		&deviceauthdomain.DeviceAuthCode{
			ID:         "code-1",
			DeviceCode: "device-code-1",
			ClientID:   "client-x",
			Verified:   false,
			Revision:   1,
		}, nil,
	)
	dacRepo.EXPECT().DeleteByDeviceCode(gomock.Any(), "device-code-1").Return(nil)

	svc := newDeviceAuthTestSvc(uow)
	err := svc.CancelDeviceAuthorization(context.Background(), DeviceCancelCommand{
		DeviceCode: "device-code-1",
		ClientID:   "client-x",
	})
	require.NoError(t, err)
}

func TestCancelDeviceAuthorization_AlreadyVerified_ReturnsConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockAuthUoW(ctrl)
	dacRepo := deviceauthmock.NewMockRepository(ctrl)
	uow.EXPECT().DeviceAuthCodes().Return(dacRepo)

	dacRepo.EXPECT().GetByDeviceCode(gomock.Any(), "device-code-1").Return(
		&deviceauthdomain.DeviceAuthCode{
			ID:         "code-1",
			DeviceCode: "device-code-1",
			ClientID:   "client-x",
			Verified:   true, // already verified — cannot cancel
			Revision:   1,
		}, nil,
	)

	svc := newDeviceAuthTestSvc(uow)
	err := svc.CancelDeviceAuthorization(context.Background(), DeviceCancelCommand{
		DeviceCode: "device-code-1",
		ClientID:   "client-x",
	})
	require.Error(t, err)
	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeConflict, target.Code)
}
