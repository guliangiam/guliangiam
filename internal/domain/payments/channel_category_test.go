package payments

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorCategoryRetryable(t *testing.T) {
	tests := []struct {
		category ErrorCategory
		retry    bool
		id       string
	}{
		{ErrorCategoryUnknown, false, "unknown"},
		{ErrorCategoryTransient, true, "transient"},
		{ErrorCategoryRateLimited, true, "rate_limited"},
		{ErrorCategoryAuth, false, "auth"},
		{ErrorCategoryPermanent, false, "permanent"},
		{ErrorCategoryConfig, false, "config"},
		{ErrorCategoryChannelOrderNotFound, false, "channel_order_not_found"},
	}
	for _, tt := range tests {
		if got := tt.category.Retryable(); got != tt.retry {
			t.Errorf("%s.Retryable() = %v, want %v", tt.id, got, tt.retry)
		}
		if got := tt.category.String(); got != tt.id {
			t.Errorf("String() = %q, want %q", got, tt.id)
		}
	}
}

func TestChannelErrorKeepsCauseChain(t *testing.T) {
	original := errors.New("upstream refused")
	chErr := &ChannelError{Code: "channel_create_failed", Message: "create failed", Cause: original}
	if !errors.Is(fmt.Errorf("outer: %w", chErr), original) {
		t.Fatal("ChannelError must expose its original error through wrapping")
	}
}
