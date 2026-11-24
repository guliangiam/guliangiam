package payments

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestChannelStatusConstants pins the shared channel-status tokens. These same
// spellings appear in several provider sub-tables, so a drift here would
// silently reclassify real payments; the values must stay byte-identical.
func TestChannelStatusConstants(t *testing.T) {
	assert.Equal(t, "SUCCESS", channelStatusSuccess)
	assert.Equal(t, "SUCCEEDED", channelStatusSucceeded)
	assert.Equal(t, "PENDING", channelStatusPending)
	assert.Equal(t, "FAILED", channelStatusFailed)
	assert.Equal(t, "CANCELLED", channelStatusCancelled)
}

// TestTranslateTradeStatus_SharedTokens covers the providers that reuse the
// shared status words (paypal, adyen, airwallex, balance, gc) — the cases the
// older per-provider tests did not exercise — plus the common fallback table.
func TestTranslateTradeStatus_SharedTokens(t *testing.T) {
	cases := []struct {
		provider string
		status   string
		want     Outcome
	}{
		{"paypal", channelStatusPending, OutcomePending},
		{"paypal", channelStatusFailed, OutcomeFailed},
		{"paypal", "COMPLETED", OutcomeSucceeded},

		{"adyen", channelStatusPending, OutcomePending},
		{"adyen", channelStatusCancelled, OutcomeCancelled},
		{"adyen", "AUTHORISED", OutcomeSucceeded},

		{"airwallex", channelStatusSuccess, OutcomeSucceeded},
		{"airwallex", channelStatusPending, OutcomePending},
		{"airwallex", channelStatusFailed, OutcomeFailed},
		{"airwallex", channelStatusCancelled, OutcomeCancelled},

		{"balance", channelStatusSucceeded, OutcomeSucceeded},
		{"balance", channelStatusPending, OutcomePending},
		{"balance", channelStatusFailed, OutcomeFailed},
		{"balance", channelStatusCancelled, OutcomeCancelled},

		{"gc", channelStatusSucceeded, OutcomeSucceeded},
		{"gc", channelStatusPending, OutcomePending},
		{"gc", channelStatusFailed, OutcomeFailed},

		// Common table: same words with an unstated provider.
		{"other", channelStatusSuccess, OutcomeSucceeded},
		{"other", channelStatusSucceeded, OutcomeSucceeded},
		{"other", channelStatusPending, OutcomePending},
	}
	for _, tc := range cases {
		t.Run(tc.provider+"_"+tc.status, func(t *testing.T) {
			assert.Equal(t, tc.want, TranslateTradeStatus(tc.provider, tc.status))
			assert.Equal(t, tc.want, TranslateTradeStatus(tc.provider, " "+tc.status+" "))
		})
	}
}

// TestTranslateTradeStatus_UnknownStillUnknown guards the fail-closed contract:
// a status outside both tables must never fabricate an outcome.
func TestTranslateTradeStatus_UnknownStillUnknown(t *testing.T) {
	assert.Equal(t, OutcomeUnknown, TranslateTradeStatus("paypal", "SOMETHING_NEW"))
	assert.Equal(t, OutcomeUnknown, TranslateTradeStatus("gc", "SOMETHING_NEW"))
	assert.Equal(t, OutcomeUnknown, TranslateTradeStatus("other", "SOMETHING_NEW"))
}
