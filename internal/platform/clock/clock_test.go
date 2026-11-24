package clock

import (
	"testing"
	"time"
)

func TestSystemClock_Now(t *testing.T) {
	c := SystemClock{}
	before := time.Now().UTC().Add(-time.Second)
	now := c.Now()
	after := time.Now().UTC().Add(time.Second)

	if now.Before(before) || now.After(after) {
		t.Errorf("SystemClock.Now() = %v, expected between %v and %v", now, before, after)
	}
}

func TestSystemClock_ImplementsClock(t *testing.T) {
	var _ Clock = SystemClock{}
}
