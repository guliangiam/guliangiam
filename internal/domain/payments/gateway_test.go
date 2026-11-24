package payments

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslateTradeStatus_Alipay(t *testing.T) {
	assert.Equal(t, OutcomeSucceeded, TranslateTradeStatus("alipay", "TRADE_SUCCESS"))
	assert.Equal(t, OutcomeSucceeded, TranslateTradeStatus("alipay", "TRADE_FINISHED"))
	assert.Equal(t, OutcomeCancelled, TranslateTradeStatus("alipay", "TRADE_CLOSED"))
	assert.Equal(t, OutcomePending, TranslateTradeStatus("alipay", "WAIT_BUYER_PAY"))
	assert.Equal(t, OutcomeUnknown, TranslateTradeStatus("alipay", "UNKNOWN"))
}

func TestTranslateTradeStatus_WeChat(t *testing.T) {
	assert.Equal(t, OutcomeSucceeded, TranslateTradeStatus("wechat", "SUCCESS"))
	assert.Equal(t, OutcomeCancelled, TranslateTradeStatus("wechat", "CLOSED"))
	assert.Equal(t, OutcomePending, TranslateTradeStatus("wechat", "NOTPAY"))
	assert.Equal(t, OutcomePending, TranslateTradeStatus("wechat", "USERPAYING"))
	assert.Equal(t, OutcomeFailed, TranslateTradeStatus("wechat", "PAYERROR"))
	assert.Equal(t, OutcomeRefunded, TranslateTradeStatus("wechat", "REFUND"))
	assert.Equal(t, OutcomeUnknown, TranslateTradeStatus("wechat", "UNKNOWN"))
}

func TestTranslateTradeStatus_Stripe(t *testing.T) {
	assert.Equal(t, OutcomeSucceeded, TranslateTradeStatus("stripe", "paid"))
	assert.Equal(t, OutcomeSucceeded, TranslateTradeStatus("stripe", "succeeded"))
	assert.Equal(t, OutcomeCancelled, TranslateTradeStatus("stripe", "canceled"))
	assert.Equal(t, OutcomePending, TranslateTradeStatus("stripe", "pending"))
	assert.Equal(t, OutcomeFailed, TranslateTradeStatus("stripe", "requires_payment_method"))
	assert.Equal(t, OutcomeUnknown, TranslateTradeStatus("stripe", "unknown"))
}

// TestTranslateTradeStatus_UnknownProvider covers the fallback used by adapters
// that do not name their channel: only values that are unambiguous across every
// channel may resolve, so an unknown provider can never widen a state change.
func TestTranslateTradeStatus_UnknownProvider(t *testing.T) {
	assert.Equal(t, OutcomeSucceeded, TranslateTradeStatus("other", "TRADE_SUCCESS"))
	assert.Equal(t, OutcomeSucceeded, TranslateTradeStatus("other", "SUCCESS"))
	assert.Equal(t, OutcomeSucceeded, TranslateTradeStatus("other", "paid"))
	assert.Equal(t, OutcomeSucceeded, TranslateTradeStatus("other", "succeeded"))
	assert.Equal(t, OutcomeCancelled, TranslateTradeStatus("other", "TRADE_CLOSED"))
	assert.Equal(t, OutcomeCancelled, TranslateTradeStatus("other", "CLOSED"))
	assert.Equal(t, OutcomeCancelled, TranslateTradeStatus("other", "canceled"))
	assert.Equal(t, OutcomePending, TranslateTradeStatus("other", "WAIT_BUYER_PAY"))
	assert.Equal(t, OutcomePending, TranslateTradeStatus("other", "NOTPAY"))
	assert.Equal(t, OutcomePending, TranslateTradeStatus("other", "pending"))
	assert.Equal(t, OutcomeUnknown, TranslateTradeStatus("other", "UNKNOWN"))
}

// TestTranslateTradeStatus_CaseAndWhitespace pins the normalization: channels
// are inconsistent about casing, and a whitespace-padded callback must not
// become an unrecognised one.
func TestTranslateTradeStatus_CaseAndWhitespace(t *testing.T) {
	assert.Equal(t, OutcomeSucceeded, TranslateTradeStatus("Alipay", " trade_success "))
	assert.Equal(t, OutcomeUnknown, TranslateTradeStatus("alipay", "   "))
}

func TestOutcome_ZeroValueIsUnknown(t *testing.T) {
	var o Outcome
	assert.Equal(t, OutcomeUnknown, o)
}
