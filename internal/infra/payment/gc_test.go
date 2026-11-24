package payment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCGateway_CreatePayment_CancelledContextAbortsBeforeHTTP(t *testing.T) {
	gw := NewGCGateway("xmpch", "secret", "https://gc.example.net")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := gw.CreatePayment(ctx, "order-1", 1.00, "CNY", "subject", "https://notify", "https://return")
	require.Error(t, err, "已取消的 context 必须在发起出站请求前中止")
}
