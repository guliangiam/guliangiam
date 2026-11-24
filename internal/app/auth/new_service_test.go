package auth

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	svc := NewService(uowMock)
	assert.NotNil(t, svc)
	assert.Equal(t, uowMock, svc.UOW)
	assert.NotNil(t, svc.Clock)
	assert.NotNil(t, svc.UUIDGenerator)
	// WebAuthn 挑战不再依赖进程内 store：begin/finish 通过 UOW 的仓储读写挑战行（31.4）。
}

func TestNewServiceWithDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	svc := NewServiceWithDependencies(clock.SystemClock{}, platformuuid.V7Generator{}, uowMock)
	assert.NotNil(t, svc)
	assert.Equal(t, uowMock, svc.UOW)
	assert.NotNil(t, svc.Clock)
	assert.NotNil(t, svc.UUIDGenerator)
}

func TestIsIgnorableLogoutError_Nil(t *testing.T) {
	assert.False(t, isIgnorableLogoutError(nil))
}

func TestIsIgnorableLogoutError_NotAppError(t *testing.T) {
	assert.False(t, isIgnorableLogoutError(assert.AnError))
}

func TestIsIgnorableLogoutError_NotFound(t *testing.T) {
	err := errors.NewNotFound("test")
	assert.True(t, isIgnorableLogoutError(err))
}

func TestIsIgnorableLogoutError_Unauthorized(t *testing.T) {
	err := errors.NewUnauthorized("test")
	assert.True(t, isIgnorableLogoutError(err))
}

func TestIsIgnorableLogoutError_OtherCode(t *testing.T) {
	err := errors.NewInvalidArgument("test")
	assert.False(t, isIgnorableLogoutError(err))
}
