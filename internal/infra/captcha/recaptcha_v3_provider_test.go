package captcha

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewReCAPTCHAV3Provider_ThresholdFallsBack(t *testing.T) {
	for _, threshold := range []float64{0, -0.1, 1.5} {
		provider := NewReCAPTCHAV3Provider("secret", threshold, "")
		assert.Equal(t, defaultReCAPTCHAV3ScoreThreshold, provider.ScoreThreshold)
	}
	provider := NewReCAPTCHAV3Provider("secret", 0.8, " signup ")
	assert.Equal(t, 0.8, provider.ScoreThreshold)
	assert.Equal(t, "signup", provider.ExpectedAction)
}

func TestReCAPTCHAV3Provider_Accept(t *testing.T) {
	provider := NewReCAPTCHAV3Provider("secret", 0.5, "")

	assert.True(t, provider.accept(true, 0.9, "login"), "score above threshold passes")
	assert.True(t, provider.accept(true, 0.5, "login"), "score equal to threshold passes")
	assert.False(t, provider.accept(true, 0.4, "login"), "score below threshold is rejected")
	assert.False(t, provider.accept(false, 0.9, "login"), "failed verification is rejected regardless of score")

	withAction := NewReCAPTCHAV3Provider("secret", 0.5, "login")
	assert.True(t, withAction.accept(true, 0.9, "login"))
	assert.False(t, withAction.accept(true, 0.9, "signup"), "a different action is rejected")
}
