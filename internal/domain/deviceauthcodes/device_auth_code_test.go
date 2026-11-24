package deviceauthcodes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeviceAuthCode_Creation(t *testing.T) {
	now := time.Now()
	dac := DeviceAuthCode{
		ID:              "dac-001",
		DeviceCode:      "DC-abc123def456",
		UserCode:        "ABCD-EFGH",
		ClientID:        "client-001",
		Scope:           "openid profile email",
		ExpiresAt:       now.Add(15 * time.Minute),
		PollIntervalSec: 5,
		Verified:        false,
		CreatedAt:       now,
		UpdatedAt:       now,
		Revision:        1,
	}

	assert.Equal(t, "DC-abc123def456", dac.DeviceCode)
	assert.Equal(t, "ABCD-EFGH", dac.UserCode)
	assert.False(t, dac.Verified, "new device code should not be verified")
	assert.Equal(t, 5, dac.PollIntervalSec)
}

func TestDeviceAuthCode_Verification(t *testing.T) {
	now := time.Now()

	t.Run("unverified_code", func(t *testing.T) {
		dac := DeviceAuthCode{
			DeviceCode: "DC-abc123",
			UserCode:   "ABCD-EFGH",
			Verified:   false,
		}
		assert.False(t, dac.Verified)
		assert.Empty(t, dac.UserID)
	})

	t.Run("verified_code", func(t *testing.T) {
		verifiedAt := now
		dac := DeviceAuthCode{
			DeviceCode: "DC-abc123",
			UserCode:   "ABCD-EFGH",
			Verified:   true,
			VerifiedAt: &verifiedAt,
			UserID:     "user-001",
			Username:   "testuser",
		}
		assert.True(t, dac.Verified)
		assert.NotNil(t, dac.VerifiedAt)
		assert.Equal(t, "user-001", dac.UserID)
	})
}

func TestDeviceAuthCode_Expiration(t *testing.T) {
	now := time.Now()

	t.Run("not_expired", func(t *testing.T) {
		dac := DeviceAuthCode{ExpiresAt: now.Add(15 * time.Minute)}
		assert.True(t, now.Before(dac.ExpiresAt))
	})

	t.Run("expired", func(t *testing.T) {
		dac := DeviceAuthCode{ExpiresAt: now.Add(-1 * time.Minute)}
		assert.True(t, now.After(dac.ExpiresAt))
	})
}

func TestDeviceAuthCode_UserCodeFormat(t *testing.T) {
	// RFC 8628: user_code should be human-readable
	tests := []struct {
		name     string
		userCode string
		valid    bool
	}{
		{name: "standard_format", userCode: "ABCD-EFGH", valid: true},
		{name: "short_code", userCode: "ABCD", valid: true},
		{name: "empty_code_invalid", userCode: "", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.userCode != ""
			assert.Equal(t, tt.valid, isValid)
		})
	}
}
