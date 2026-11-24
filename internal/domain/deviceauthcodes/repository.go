package deviceauthcodes

import (
	"context"
	"time"
)

// Repository defines the interface for device authorization code persistence.
type Repository interface {
	// GetByDeviceCode retrieves a device auth code by device_code.
	GetByDeviceCode(ctx context.Context, deviceCode string) (*DeviceAuthCode, error)

	// GetByUserCode retrieves a device auth code by user_code.
	GetByUserCode(ctx context.Context, userCode string) (*DeviceAuthCode, error)

	// Create persists a new device authorization code.
	Create(ctx context.Context, code *DeviceAuthCode) error

	// UpdateVerification updates the verification status and user info.
	UpdateVerification(ctx context.Context, code *DeviceAuthCode) error

	// RecordPoll stamps the poll time and increments the slow_down counter,
	// returning the new counter value. The increment lives in the repository so
	// it stays atomic across concurrent polls of the same device_code — which is
	// the entire point of the RFC 8628 §3.5 throttle.
	RecordPoll(ctx context.Context, deviceCode string, polledAt time.Time) (int, error)

	// ResetSlowDown clears the consecutive-violation counter after a compliant
	// poll. The cap is about consecutive violations, not lifetime ones.
	ResetSlowDown(ctx context.Context, deviceCode string, polledAt time.Time) error

	// DeleteByDeviceCode removes a device auth code by device_code.
	DeleteByDeviceCode(ctx context.Context, deviceCode string) error

	// DeleteByUserCode removes a device auth code by user_code.
	DeleteByUserCode(ctx context.Context, userCode string) error

	// DeleteExpired removes all expired device auth codes.
	DeleteExpired(ctx context.Context) (int64, error)
}
