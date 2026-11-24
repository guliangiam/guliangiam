package payments

import (
	"context"
	"errors"
	"testing"
	"time"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRefundExecutor struct {
	refund *domain.Refund
	err    error
	gotID  string
	gotOrg string
	gotAmt float64
}

func (s *stubRefundExecutor) ExecuteRefund(_ context.Context, paymentID, organizationID string, amount float64, _ string) (*domain.Refund, error) {
	s.gotID, s.gotOrg, s.gotAmt = paymentID, organizationID, amount
	return s.refund, s.err
}

// BE-C-09：未接线时 refund 端点必须返回 501 而不是 panic/静默成功。
func TestRefundPayment_NotWired(t *testing.T) {
	handler := Handler{}
	_, err := handler.RefundPayment(context.Background(), "org-1", "pay-1", 5, "")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 501, appErr.HTTPStatus())
}

func TestRefundPayment_MapsDetailAndTrustedOrg(t *testing.T) {
	now := time.Now().UTC()
	stub := &stubRefundExecutor{refund: &domain.Refund{
		ID: "ref-1", PaymentID: "pay-1", OrganizationID: "org-1",
		OutRefundNo: "ref-1", Channel: "stripe", Amount: 5, Currency: "CNY",
		State: domain.RefundStateSucceeded, Reason: "duplicate",
		RequestedAt: now, CreatedAt: now, UpdatedAt: now,
	}}
	handler := Handler{Refund: stub}

	detail, err := handler.RefundPayment(context.Background(), "org-1", "pay-1", 5, "duplicate")
	require.NoError(t, err)
	assert.Equal(t, "ref-1", detail.ID)
	assert.Equal(t, "pay-1", detail.PaymentID)
	assert.Equal(t, domain.RefundStateSucceeded, detail.State)
	assert.Equal(t, 5.0, detail.Amount)
	// 编排器收到的是可信租户与路径 id，而不是请求体声称值。
	assert.Equal(t, "pay-1", stub.gotID)
	assert.Equal(t, "org-1", stub.gotOrg)
	assert.Equal(t, 5.0, stub.gotAmt)
}

func TestRefundPayment_PropagatesOrchestratorError(t *testing.T) {
	stub := &stubRefundExecutor{err: apperrors.NewUnprocessableEntityf("refund amount exceeds available")}
	handler := Handler{Refund: stub}
	_, err := handler.RefundPayment(context.Background(), "org-1", "pay-1", 999, "")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 422, appErr.HTTPStatus())
}

var _ apppayments.RefundExecutor = (*stubRefundExecutor)(nil)
