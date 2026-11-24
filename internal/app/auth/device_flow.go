package auth

import (
	"context"
	"log/slog"
	"strings"
	"time"

	deviceauthcodesdomain "github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
)

// Device-grant protocol error codes (RFC 8628 §3.5). They belong to the protocol
// namespace (错误码注册表 §2), not to the application error table: a device client
// dispatches on these exact strings, and the previous private code
// `invalid_device_code` matched nothing in the RFC.
const (
	deviceErrorAuthorizationPending = "authorization_pending"
	deviceErrorSlowDown             = "slow_down"
	deviceErrorExpiredToken         = "expired_token"
	deviceErrorAccessDenied         = "access_denied"
)

// deviceProtocolError carries an RFC 8628 code to the protocol face. It
// implements platform/errors.ProtocolCodeError, the same bridge the authorize
// endpoint uses for `unsupported_response_type`.
type deviceProtocolError struct {
	code        string
	description string
}

func (e *deviceProtocolError) Error() string {
	if e.description == "" {
		return e.code
	}
	return e.code + ": " + e.description
}

func (e *deviceProtocolError) ProtocolCode() string { return e.code }
func (e *deviceProtocolError) HTTPStatus() int      { return 400 }

func newDeviceProtocolError(code, description string) error {
	return &deviceProtocolError{code: code, description: description}
}

// deviceSlowDownInterval is the extra wait RFC 8628 §3.5 tells a client to add
// after each `slow_down`. It is reported in the error description so the client
// has an actionable number rather than a bare code.
const deviceSlowDownInterval = 5 * time.Second

// maxDeviceSlowDownCount terminates a poll that keeps arriving too fast. Five
// consecutive violations is well beyond clock skew or a single retry burst.
const maxDeviceSlowDownCount = 5

// enforceDevicePollInterval applies the RFC 8628 §3.5 polling rule.
//
// The counter lives in the database and is incremented by the repository in one
// statement, so concurrent polls of the same device_code cannot each observe the
// same value and slip past the cap.
func (s *Service) enforceDevicePollInterval(ctx context.Context, entry *deviceauthcodesdomain.DeviceAuthCode, now time.Time) error {
	interval := time.Duration(entry.PollIntervalSec) * time.Second
	if interval <= 0 {
		interval = defaultDevicePollInterval
	}

	tooFast := entry.LastPolledAt != nil && now.Sub(*entry.LastPolledAt) < interval

	if !tooFast {
		// A compliant poll clears the strike count: the cap is about consecutive
		// violations, not lifetime ones.
		if err := s.UOW.DeviceAuthCodes().ResetSlowDown(ctx, entry.DeviceCode, now); err != nil {
			slog.WarnContext(ctx, "device flow: could not record compliant poll", "error", err)
		}
		return nil
	}

	count, err := s.UOW.DeviceAuthCodes().RecordPoll(ctx, entry.DeviceCode, now)
	if err != nil {
		// Failing open here would silently disable the throttle, so the poll is
		// refused instead; the client simply retries.
		return newDeviceProtocolError(deviceErrorSlowDown, "polling too frequently")
	}

	if count >= maxDeviceSlowDownCount {
		_ = s.UOW.DeviceAuthCodes().DeleteByDeviceCode(ctx, entry.DeviceCode)
		slog.WarnContext(ctx, "device flow: polling abuse, authorization terminated",
			"client_id", entry.ClientID,
			"slow_down_count", count,
		)
		return newDeviceProtocolError(deviceErrorExpiredToken, "device code was terminated after repeated rapid polling")
	}

	return newDeviceProtocolError(deviceErrorSlowDown, "poll again after "+deviceSlowDownInterval.String())
}

// defaultDevicePollInterval mirrors the RFC 8628 §3.4 default when a row carries
// no usable interval.
const defaultDevicePollInterval = 5 * time.Second

// verificationURI builds the absolute URL a device shows to the user
// (RFC 8628 §3.3.1). It is derived from the same issuer as every other absolute
// URL (14.13.1); a relative path would leave the user with nothing to type.
func verificationURI(issuer string) string {
	base := strings.TrimRight(strings.TrimSpace(issuer), "/")
	if base == "" {
		return ""
	}
	return base + "/device"
}

// verificationURIComplete appends the user_code so a QR code can carry it. The
// verification page must still display the code for the user to confirm
// (RFC 8628 §5.4: the URL is attacker-controllable if the QR is replaced).
func verificationURIComplete(issuer, userCode string) string {
	base := verificationURI(issuer)
	if base == "" || strings.TrimSpace(userCode) == "" {
		return ""
	}
	return base + "?user_code=" + userCode
}
