package auth

import (
	"context"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

func (h Handler) BeginTrustedDeviceRegistration(ctx context.Context, command appauth.TrustedDeviceBeginCommand) (*appauth.TrustedDeviceBeginResult, error) {
	return h.Service.BeginTrustedDeviceRegistration(ctx, command)
}

func (h Handler) RegisterTrustedDevice(ctx context.Context, command appauth.TrustedDeviceRegisterCommand) (*appauth.TrustedDeviceView, error) {
	return h.Service.RegisterTrustedDevice(ctx, command)
}

func (h Handler) ListTrustedDevices(ctx context.Context, organizationID, userID string) ([]appauth.TrustedDeviceView, error) {
	return h.Service.ListTrustedDevices(ctx, organizationID, userID)
}

func (h Handler) RevokeTrustedDevice(ctx context.Context, organizationID, userID, deviceID string) error {
	return h.Service.RevokeTrustedDevice(ctx, organizationID, userID, deviceID)
}
