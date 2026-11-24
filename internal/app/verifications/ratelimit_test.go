package verifications

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter()
	assert.NotNil(t, rl)
	rl.Stop()
}

func TestRateLimiter_CheckAttempt_NoRecord(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	assert.True(t, rl.CheckAttempt("email", "test@example.com"))
}

func TestRateLimiter_RecordAndCheckAttempt(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	// Record failed attempts up to the limit
	for i := 0; i < MaxVerifyAttempts; i++ {
		rl.RecordFailedAttempt("email", "test@example.com")
	}
	// Should be blocked now
	assert.False(t, rl.CheckAttempt("email", "test@example.com"))
}

func TestRateLimiter_ResetAttempts(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	for i := 0; i < MaxVerifyAttempts; i++ {
		rl.RecordFailedAttempt("email", "test@example.com")
	}
	rl.ResetAttempts("email", "test@example.com")
	assert.True(t, rl.CheckAttempt("email", "test@example.com"))
}

func TestRateLimiter_CheckIP_NoRecord(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	assert.True(t, rl.CheckIP("192.168.1.1"))
}

func TestRateLimiter_CheckIP_Empty(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	assert.True(t, rl.CheckIP(""))
}

func TestRateLimiter_RecordAndCheckIP(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	for i := 0; i < MaxIPVerifyAttempts; i++ {
		rl.RecordIPAttempt("192.168.1.1")
	}
	assert.False(t, rl.CheckIP("192.168.1.1"))
}

func TestRateLimiter_RecordIPAttempt_EmptyIP(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	rl.RecordIPAttempt("") // should be a no-op
	assert.True(t, rl.CheckIP(""))
}

func TestRateLimiter_DifferentRecipients(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	for i := 0; i < MaxVerifyAttempts; i++ {
		rl.RecordFailedAttempt("email", "user1@example.com")
	}
	// Different recipient should still be allowed
	assert.True(t, rl.CheckAttempt("email", "user2@example.com"))
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	for i := 0; i < MaxIPVerifyAttempts; i++ {
		rl.RecordIPAttempt("192.168.1.1")
	}
	// Different IP should still be allowed
	assert.True(t, rl.CheckIP("192.168.1.2"))
}

func TestRateLimiter_ExpiredAttempt(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	// Set a custom clock that we can advance
	fixedTime := time.Now()
	rl.clock = func() time.Time { return fixedTime }

	for i := 0; i < MaxVerifyAttempts; i++ {
		rl.RecordFailedAttempt("email", "test@example.com")
	}
	assert.False(t, rl.CheckAttempt("email", "test@example.com"))

	// Advance time past the expiry
	fixedTime = fixedTime.Add(verificationCodeTimeout + time.Second)
	assert.True(t, rl.CheckAttempt("email", "test@example.com"))
}

func TestRateLimiter_ExpiredIP(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	fixedTime := time.Now()
	rl.clock = func() time.Time { return fixedTime }

	for i := 0; i < MaxIPVerifyAttempts; i++ {
		rl.RecordIPAttempt("192.168.1.1")
	}
	assert.False(t, rl.CheckIP("192.168.1.1"))

	// Advance time past the expiry
	fixedTime = fixedTime.Add(ipRateLimitWindow + time.Second)
	assert.True(t, rl.CheckIP("192.168.1.1"))
}
