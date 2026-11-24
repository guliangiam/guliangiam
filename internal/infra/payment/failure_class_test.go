package payment

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v82"
)

func TestUpstreamStatusClassification(t *testing.T) {
	tests := []struct {
		status   int
		category domain.ErrorCategory
		code     apperrors.Code
	}{
		{http.StatusTooManyRequests, domain.ErrorCategoryRateLimited, apperrors.CodeTooManyRequests},
		{http.StatusUnauthorized, domain.ErrorCategoryAuth, apperrors.CodeUnauthorized},
		{http.StatusForbidden, domain.ErrorCategoryAuth, apperrors.CodeForbidden},
		{http.StatusRequestTimeout, domain.ErrorCategoryTransient, apperrors.CodeUnavailable},
		{http.StatusBadGateway, domain.ErrorCategoryTransient, apperrors.CodeUnavailable},
		{http.StatusServiceUnavailable, domain.ErrorCategoryTransient, apperrors.CodeUnavailable},
		{http.StatusGatewayTimeout, domain.ErrorCategoryTransient, apperrors.CodeUnavailable},
		{http.StatusBadRequest, domain.ErrorCategoryPermanent, apperrors.CodeInvalidArgument},
		{http.StatusNotFound, domain.ErrorCategoryPermanent, apperrors.CodeNotFound},
		{http.StatusConflict, domain.ErrorCategoryPermanent, apperrors.CodeConflict},
		{http.StatusUnprocessableEntity, domain.ErrorCategoryPermanent, apperrors.CodeUnprocessableEntity},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.category, categoryForStatus(tt.status), http.StatusText(tt.status))
		assert.Equal(t, tt.code, newUpstreamStatusErrorf(tt.status, nil, "channel failure").Code)
	}
}

func TestRetryAfterFromUpstreamHeader(t *testing.T) {
	for _, tt := range []struct {
		value string
		secs  int
	}{{"7", 7}, {" 7 ", 7}} {
		header := http.Header{}
		header.Set("Retry-After", tt.value)
		seconds, ok := retryAfterSeconds(header)
		assert.True(t, ok, tt.value)
		assert.Equal(t, tt.secs, seconds)
	}
	for _, value := range []string{"0", "-3", "1.5", "Wed, 21 Oct 2015 07:28:00 GMT", "1s", ""} {
		header := http.Header{}
		header.Set("Retry-After", value)
		if _, ok := retryAfterSeconds(header); ok {
			t.Fatalf("Retry-After %q must be dropped", value)
		}
	}
}

func TestClassifyError(t *testing.T) {
	dialFailure := &url.Error{Op: "Get", URL: "https://api.example.com",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}}
	tests := []struct {
		err  error
		want domain.ErrorCategory
	}{
		{nil, domain.ErrorCategoryUnknown},
		{errors.New("boom"), domain.ErrorCategoryUnknown},
		{apperrors.NewTooManyRequests("limited"), domain.ErrorCategoryRateLimited},
		{apperrors.NewUnavailable("down"), domain.ErrorCategoryTransient},
		{apperrors.NewUnauthorized("rejected"), domain.ErrorCategoryAuth},
		{apperrors.NewForbidden("denied"), domain.ErrorCategoryAuth},
		{apperrors.NewInvalidArgument("bad request"), domain.ErrorCategoryPermanent},
		{apperrors.NewNotFound("no such order"), domain.ErrorCategoryChannelOrderNotFound},
		{apperrors.NewUnimplemented("unsupported"), domain.ErrorCategoryPermanent},
		{fmt.Errorf("gateway: %w", apperrors.NewTooManyRequests("limited")), domain.ErrorCategoryRateLimited},
		{dialFailure, domain.ErrorCategoryTransient},
		{context.DeadlineExceeded, domain.ErrorCategoryTransient},
		{context.Canceled, domain.ErrorCategoryUnknown},
		{&stripe.Error{HTTPStatusCode: http.StatusTooManyRequests}, domain.ErrorCategoryRateLimited},
		{&stripe.Error{HTTPStatusCode: http.StatusServiceUnavailable}, domain.ErrorCategoryTransient},
		{&stripe.Error{HTTPStatusCode: http.StatusPaymentRequired}, domain.ErrorCategoryPermanent},
	}
	for _, tt := range tests {
		require.NotPanics(t, func() { assert.Equal(t, tt.want, classifyError(tt.err), fmt.Sprintf("%v", tt.err)) })
	}
}

func TestNewChannelErrorKeepsChainAndRetryAfter(t *testing.T) {
	chErr := newChannelError("channel_create_failed",
		newUpstreamStatusErrorf(http.StatusTooManyRequests, http.Header{"Retry-After": {"5"}}, "channel rate limited"))

	assert.Equal(t, "channel_create_failed", chErr.Code)
	assert.Equal(t, domain.ErrorCategoryRateLimited, chErr.Category)
	require.NotNil(t, chErr.RetryAfter)
	assert.Equal(t, 5*time.Second, *chErr.RetryAfter)

	// 无法解析的上游值只丢弃 Retry-After，分类保持 rate_limited。
	unparsable := newChannelError("channel_query_failed",
		newUpstreamStatusErrorf(http.StatusTooManyRequests, http.Header{"Retry-After": {"1.5"}}, "limited"))
	assert.Equal(t, domain.ErrorCategoryRateLimited, unparsable.Category)
	assert.Nil(t, unparsable.RetryAfter)
}

func TestConfigFailureIsNotRetryable(t *testing.T) {
	stripeCert := NewChannelAdapter(channelStripe, failingPaymentGateway{})
	for _, err := range []error{
		configFailure(apperrors.NewInvalidArgument("apiKey is required")),
		stripeCert.VerifyConfig(context.Background(), domain.ChannelConfig{}),
	} {
		retryable, ok := apperrors.IsRetryable(err)
		assert.True(t, ok, "config failures must carry an explicit retry marker")
		assert.False(t, retryable)
	}
	assert.Nil(t, configFailure(nil))
}

// failingPaymentGateway returns a fixed error from every vendor call.
type failingPaymentGateway struct{ err error }

func (g failingPaymentGateway) CreatePayment(context.Context, string, float64, string, string, string, string) (*PaymentURLResult, error) {
	return nil, g.err
}
func (g failingPaymentGateway) QueryPayment(context.Context, string) (*PaymentStatusResult, error) {
	return nil, g.err
}
func (g failingPaymentGateway) Refund(context.Context, string, float64, string) (*RefundResult, error) {
	return nil, g.err
}
func (g failingPaymentGateway) VerifyNotify(context.Context, map[string]string) (*NotifyResult, error) {
	return nil, g.err
}

// 每个注册渠道共享 ChannelAdapter 边界，必须对同一组输入给出相同分类
// （每渠道 ≥5 类：rate_limited / transient / auth / permanent / unknown）。
func TestChannelAdapterClassifiesEachChannel(t *testing.T) {
	inputs := []struct {
		name string
		err  error
		want domain.ErrorCategory
	}{
		{"rate limited", newUpstreamStatusErrorf(http.StatusTooManyRequests, nil, "limited"), domain.ErrorCategoryRateLimited},
		{"upstream 5xx", newUpstreamStatusErrorf(http.StatusBadGateway, nil, "bad gateway"), domain.ErrorCategoryTransient},
		{"timeout", &url.Error{Op: "POST", URL: "https://api.example.com",
			Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("i/o timeout")}}, domain.ErrorCategoryTransient},
		{"credential rejected", newUpstreamStatusErrorf(http.StatusUnauthorized, nil, "unauthorized"), domain.ErrorCategoryAuth},
		{"bad request", newUpstreamStatusErrorf(http.StatusUnprocessableEntity, nil, "unprocessable"), domain.ErrorCategoryPermanent},
		{"opaque", errors.New("unexpected"), domain.ErrorCategoryUnknown},
	}
	for _, channel := range ChannelNames() {
		for _, input := range inputs {
			adapter := NewChannelAdapter(channel, failingPaymentGateway{err: input.err})
			_, chErr := adapter.Charge(context.Background(), domain.ChargeRequest{OutTradeNo: "order-1"})
			require.NotNil(t, chErr, channel+"/"+input.name)
			assert.Equal(t, input.want, chErr.Category, channel+"/"+input.name)
			assert.ErrorIs(t, chErr, input.err)
		}
	}
}

func TestChannelAdapterOtherErrorPoints(t *testing.T) {
	rateLimited := newUpstreamStatusErrorf(http.StatusTooManyRequests, nil, "limited")
	adapter := NewChannelAdapter(channelStripe, failingPaymentGateway{err: rateLimited})

	_, queryErr := adapter.Query(context.Background(), domain.QueryRequest{OutTradeNo: "order-1"})
	require.NotNil(t, queryErr)
	assert.Equal(t, domain.ErrorCategoryRateLimited, queryErr.Category)

	_, refundErr := adapter.Refund(context.Background(), domain.RefundRequest{ChannelOrderNo: "order-1"})
	require.NotNil(t, refundErr)
	assert.Equal(t, domain.ErrorCategoryRateLimited, refundErr.Category)

	_, queryRefundErr := adapter.QueryRefund(context.Background(), domain.QueryRefundRequest{})
	_, closeErr := adapter.Close(context.Background(), domain.CloseRequest{})
	_, statementErr := adapter.DownloadStatement(context.Background(), domain.StatementRequest{})
	for _, unsupportedErr := range []*domain.ChannelError{queryRefundErr, closeErr, statementErr} {
		require.NotNil(t, unsupportedErr)
		assert.Equal(t, domain.ErrorCategoryPermanent, unsupportedErr.Category)
	}
}

// 端到端：真实 HTTP 网关必须把 429 + Retry-After 转成 rate_limited 与秒数。
func TestUpstreamStatusProducerEndToEnd(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "6")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	gateway := NewPaddleGateway("pdl_test_token", "whsec_test", true)
	gateway.BaseURL = server.URL
	gateway.client = server.Client()

	_, chErr := NewChannelAdapter(channelPaddle, gateway).Charge(
		context.Background(), domain.ChargeRequest{OutTradeNo: "order-1", Amount: 1})
	require.NotNil(t, chErr)
	assert.Equal(t, domain.ErrorCategoryRateLimited, chErr.Category)
	require.NotNil(t, chErr.RetryAfter)
	assert.Equal(t, 6*time.Second, *chErr.RetryAfter)
}
