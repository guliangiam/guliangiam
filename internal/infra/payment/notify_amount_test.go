package payment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 渠道回调携带非法金额时必须在解析处失败，而不是静默折算为 0：
// 0 金额会直接污染对账差异与退款金额计算。
func TestVerifyNotify_RejectsInvalidAmount(t *testing.T) {
	tests := []struct {
		name    string
		gateway interface {
			VerifyNotify(context.Context, map[string]string) (*NotifyResult, error)
		}
		params   map[string]string
		wantHint string
	}{
		{
			name:     "wechat",
			gateway:  &WeChatPayGateway{},
			params:   map[string]string{"out_trade_no": "pay-1", "trade_state": "SUCCESS", "total": "abc"},
			wantHint: "invalid total",
		},
		{
			name:     "stripe",
			gateway:  &StripeGateway{},
			params:   map[string]string{"out_trade_no": "pay-1", "event_type": "checkout.session.completed", "amount_total": "12.5"},
			wantHint: "invalid amount_total",
		},
		{
			name:     "paypal",
			gateway:  &PayPalGateway{},
			params:   map[string]string{fieldOrderID: "pay-1", fieldAmount: "x"},
			wantHint: "invalid amount",
		},
		{
			name:     "fastspring",
			gateway:  &FastSpringGateway{},
			params:   map[string]string{fieldOrderID: "pay-1", "total": "n/a"},
			wantHint: "invalid total",
		},
		{
			name:     "lemonsqueezy",
			gateway:  &LemonSqueezyGateway{},
			params:   map[string]string{fieldOrderID: "pay-1", "total": "n/a"},
			wantHint: "invalid total",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.gateway.VerifyNotify(context.Background(), tc.params)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tc.wantHint)
		})
	}
}

// 合法金额仍按渠道单位换算：微信/Stripe 用「分」，其余用「元」。
func TestVerifyNotify_AcceptsValidAmount(t *testing.T) {
	tests := []struct {
		name    string
		gateway interface {
			VerifyNotify(context.Context, map[string]string) (*NotifyResult, error)
		}
		params map[string]string
		want   float64
	}{
		{
			name:    "wechat_cents",
			gateway: &WeChatPayGateway{},
			params:  map[string]string{"out_trade_no": "pay-1", "trade_state": "SUCCESS", "total": "1050"},
			want:    10.5,
		},
		{
			name:    "stripe_cents",
			gateway: &StripeGateway{},
			params:  map[string]string{"out_trade_no": "pay-1", "amount_total": "1050"},
			want:    10.5,
		},
		{
			name:    "paypal_amount",
			gateway: &PayPalGateway{},
			params:  map[string]string{fieldOrderID: "pay-1", fieldAmount: "10.50"},
			want:    10.50,
		},
		{
			name:    "fastspring_total",
			gateway: &FastSpringGateway{},
			params:  map[string]string{fieldOrderID: "pay-1", "total": "10.50"},
			want:    10.50,
		},
		{
			name:    "lemonsqueezy_total",
			gateway: &LemonSqueezyGateway{},
			params:  map[string]string{fieldOrderID: "pay-1", "total": "10.50"},
			want:    10.50,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.gateway.VerifyNotify(context.Background(), tc.params)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tc.want, result.Amount)
		})
	}
}
