package notifications

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateRetryDelay(t *testing.T) {
	w := &Worker{BaseRetryDelay: time.Second}

	delay := w.calculateRetryDelay(1)
	assert.Equal(t, time.Second, delay)

	delay = w.calculateRetryDelay(2)
	assert.Equal(t, 2*time.Second, delay)

	delay = w.calculateRetryDelay(3)
	assert.Equal(t, 4*time.Second, delay)

	// attempt=10: 1s * 1<<9 = 512s = 8m32s (not yet capped)
	delay = w.calculateRetryDelay(10)
	assert.Equal(t, 512*time.Second, delay)

	// With larger base delay, capped at max
	w2 := &Worker{BaseRetryDelay: time.Minute}
	// attempt=7: 60s * 1<<6 = 60s * 64 = 3840s = 64min > 1h, capped
	delay = w2.calculateRetryDelay(7)
	assert.Equal(t, workerMaxRetryDelay, delay)
}

func TestNewWorker_Defaults(t *testing.T) {
	w := &Worker{BaseRetryDelay: workerDefaultBaseRetryDelay, MaxRetries: workerDefaultMaxRetries}
	assert.Equal(t, 60*time.Second, w.BaseRetryDelay)
	assert.Equal(t, 3, w.MaxRetries)
}
