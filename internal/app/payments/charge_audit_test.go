package payments

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	pmock "github.com/guliangiam/guliangiam/internal/domain/payments/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Charge 成功路径必须在同一事务内写审计（action=payment.captured / event=payment.captured）。
func TestChargeOrchestrator_Execute_WritesAuditInSameTx(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := pmock.NewMockRepository(ctrl)
	payment := &domain.Payment{
		ID: "p-audit-1", OrganizationID: "org-1", Price: 10,
		Provider: "stripe", State: domain.StateCreated,
	}
	gw := &stubChannelGateway{name: "stripe", result: &domain.ChargeResult{State: domain.AttemptStateSucceeded, ChannelOrderNo: "ch-1", PayURL: "https://pay"}}
	reg := NewGatewayRegistry()
	reg.Register("stripe", gw)

	repo.EXPECT().GetByID(gomock.Any(), "p-audit-1").Return(payment, nil).Times(3)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	runner := newStubChargeRunner(repo)
	orch := NewChargeOrchestrator(clock.SystemClock{}, platformuuid.V7Generator{}, repo, reg, stubChannelConfigs{cfg: activeChannelConfig()}, runner)

	got, err := orch.Execute(context.Background(), "p-audit-1", "org-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	// BE-C-10：认领（payment.updated）+ 结果（payment.captured）各一条审计。
	require.Len(t, runner.tx.audits, 2)
	assert.Equal(t, "payment.updated", runner.tx.audits[0].EventType)
	audit := runner.tx.audits[1]
	assert.Equal(t, "payment.captured", audit.Action)
	assert.Equal(t, "payment.captured", audit.EventType)
	assert.Equal(t, "p-audit-1", audit.ResourceID)
	assert.Equal(t, "org-1", audit.OrganizationID)
}

// 渠道失败路径同样在事务内更新 state=failed 并写 payment.charge.failed 审计。
func TestChargeOrchestrator_Execute_FailureWritesAudit(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := pmock.NewMockRepository(ctrl)
	payment := &domain.Payment{
		ID: "p-audit-2", OrganizationID: "org-1", Price: 10,
		Provider: "stripe", State: domain.StateCreated,
	}
	gw := &stubChannelGateway{name: "stripe", cErr: &domain.ChannelError{Code: "TIMEOUT", Message: "timeout"}}
	reg := NewGatewayRegistry()
	reg.Register("stripe", gw)

	repo.EXPECT().GetByID(gomock.Any(), "p-audit-2").Return(payment, nil).Times(3)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	runner := newStubChargeRunner(repo)
	orch := NewChargeOrchestrator(clock.SystemClock{}, platformuuid.V7Generator{}, repo, reg, stubChannelConfigs{cfg: activeChannelConfig()}, runner)

	_, err := orch.Execute(context.Background(), "p-audit-2", "org-1")
	require.Error(t, err)
	// 认领审计 + 失败审计。
	require.Len(t, runner.tx.audits, 2)
	audit := runner.tx.audits[1]
	assert.Equal(t, "payment.updated", audit.Action)
	assert.Equal(t, "payment.charge.failed", audit.EventType)
	assert.Equal(t, "TIMEOUT", audit.Payload["errorCode"])
}
