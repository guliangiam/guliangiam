// Package mfatotp is the single TOTP verification implementation shared by
// login and MFA enrollment. Keeping one implementation guarantees the setup
// proof uses the same ±1 window and replay model as an actual login.
package mfatotp

import (
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Period is the TOTP time step used across login and enrollment: the RFC 6238
// §4.1 default of 30 s (rooted in RFC 4226 §4 T0), with the §5.4 6-digit
// SHA-1 profile below.
const Period = 30 * time.Second

// MatchStep returns the time-step that produced the passcode. The candidate
// window {step-1, step, step+1} is the RFC 6238 §5.2 recommended ±1 step
// acceptance range; DigitsSix + AlgorithmSHA1 are the RFC 6238 §5.1/§5.4
// defaults. Validating each candidate with Skew 0 (instead of one call with
// Skew 1) is what makes the matched window observable for replay detection.
func MatchStep(secret, passcode string, now time.Time) (int64, bool, error) {
	trimmedSecret := strings.TrimSpace(secret)
	trimmedPasscode := strings.TrimSpace(passcode)
	if trimmedPasscode == "" || trimmedSecret == "" {
		return 0, false, nil
	}

	baseStep := now.UTC().Unix() / int64(Period/time.Second)
	for _, step := range []int64{baseStep - 1, baseStep, baseStep + 1} {
		valid, err := totp.ValidateCustom(trimmedPasscode, trimmedSecret, StepStart(step), totp.ValidateOpts{
			Period:    uint(Period / time.Second),
			Skew:      0,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			return 0, false, err
		}
		if valid {
			return step, true, nil
		}
	}
	return 0, false, nil
}

// StepStart is the UTC start of a TOTP time-step.
func StepStart(step int64) time.Time {
	return time.Unix(step*int64(Period/time.Second), 0).UTC()
}

// Replayed reports whether a passcode from matchedAt was already consumed.
// RFC 6238 §5.2 requires at most one acceptance per time step; remembering the
// step that produced the last accepted code closes the ±1-window reuse path.
func Replayed(lastUsed *time.Time, matchedAt time.Time) bool {
	if lastUsed == nil {
		return false
	}
	windowEnd := matchedAt.Add(Period)
	return !lastUsed.Before(matchedAt) && lastUsed.Before(windowEnd.Add(Period))
}
