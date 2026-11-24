package users

import (
	"context"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ListUserTrustedDevices returns the trusted devices of the user resolved from
// {id} within the trusted tenant scope.
func (h Handler) ListUserTrustedDevices(ctx context.Context, organizationID, userID string) ([]appauth.TrustedDeviceView, error) {
	if h.AuthService == nil {
		return nil, apperrors.NewInternal("trusted device service unavailable")
	}
	return h.AuthService.ListTrustedDevices(ctx, organizationID, userID)
}

// DeleteUserTrustedDevice revokes a device belonging to the user resolved from
// {id}. A device owned by another user or tenant is reported as 404.
func (h Handler) DeleteUserTrustedDevice(ctx context.Context, organizationID, userID, deviceID string) error {
	if h.AuthService == nil {
		return apperrors.NewInternal("trusted device service unavailable")
	}
	return h.AuthService.RevokeTrustedDevice(ctx, organizationID, userID, deviceID)
}
