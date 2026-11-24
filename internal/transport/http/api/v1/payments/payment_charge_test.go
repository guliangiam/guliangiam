package payments

import (
	"context"
	"errors"
	"testing"

	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	pmock "github.com/guliangiam/guliangiam/internal/domain/payments/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type draftConfigResolver struct{}

func (draftConfigResolver) ResolveActive(ctx context.Context, organizationID, channel string) (*domain.PayChannelConfig, error) {
	return &domain.PayChannelConfig{Channel: channel, OrganizationID: organizationID, Environment: "production", Status: domain.ChannelConfigStatusDraft}, nil
}

// AC-PAY-006：渠道配置为 draft 时，Charge 在传输层边界返回 422
// payment_channel_not_activated（编排内部完成激活校验，不触达渠道）。
func TestChargePayment_DraftConfigRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := pmock.NewMockRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), "pay-1").Return(&domain.Payment{
		ID: "pay-1", OrganizationID: "org-1", State: domain.StateCreated, Provider: "stripe", Price: 10,
	}, nil)

	orchestrator := apppayments.NewChargeOrchestrator(clock.SystemClock{}, platformuuid.V7Generator{}, repo, apppayments.NewGatewayRegistry(), draftConfigResolver{}, nil)
	handler := Handler{Charge: orchestrator}

	_, err := handler.ChargePayment(context.Background(), "org-1", "pay-1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodePaymentChannelNotActivated, appErr.Code)
	assert.Equal(t, 422, appErr.HTTPStatus())
}

func TestChargePayment_NotWired(t *testing.T) {
	handler := Handler{}
	_, err := handler.ChargePayment(context.Background(), "org-1", "pay-1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperrors.CodeUnimplemented, appErr.Code)
}
