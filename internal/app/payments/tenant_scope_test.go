package payments

import (
	"context"
	"errors"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	repoMock "github.com/guliangiam/guliangiam/internal/domain/payments/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func requireAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	require.Error(t, err)
	appErr := apperrors.FromError(err)
	assert.Equal(t, code, appErr.Code)
}

// TestPaymentByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the app layer rejects before touching any repository.
func TestPaymentByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetPayment(ctx, "  ", "p-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	err = svc.PatchPayment(ctx, "", PatchPaymentCommand{ID: "p-1"})
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	err = svc.DeletePayment(ctx, "", "p-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	err = svc.InvoicePayment(ctx, "", InvoicePaymentCommand{ID: "p-1"})
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	_, err = svc.GetPaymentResult(ctx, "", "p-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
}

// TestGetPayment_PassesTrustedOrganization pins the exact tenant argument seen
// by the repository.
func TestGetPayment_PassesTrustedOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Payments().Return(repo)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "p-1", "org-1").
		Return(&domain.Payment{ID: "p-1", OrganizationID: "org-1"}, nil)

	_, err := svc.GetPayment(context.Background(), "org-1", "p-1")
	require.NoError(t, err)
}

// TestPatchPayment_ForeignTenantStopsBeforeUpdate pins that ownership is decided
// before mutation: a NotFound read leaves Update uncalled and writes no audit.
func TestPatchPayment_ForeignTenantStopsBeforeUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Payments().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "p-1", "org-1").
		Return(nil, apperrors.NewNotFound("payment not found"))
	// No Update expectation: a foreign id must never reach the write.

	displayName := "hijacked"
	err := svc.PatchPayment(context.Background(), "org-1", PatchPaymentCommand{ID: "p-1", DisplayName: &displayName})
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

// TestDeletePayment_ForeignTenantStopsBeforeDelete pins that a foreign id is
// rejected before Delete and the audit row carries the entity organization.
func TestDeletePayment_ForeignTenantStopsBeforeDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Payments().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "p-1", "org-1").
		Return(nil, apperrors.NewNotFound("payment not found"))
	// No Delete expectation: a foreign id must never be removed.

	err := svc.DeletePayment(context.Background(), "org-1", "p-1")
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

// TestDeletePayment_PassesEntityOrganizationToDelete pins the successful
// same-tenant delete and the repository arguments.
func TestDeletePayment_PassesEntityOrganizationToDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: uow}

	uow.EXPECT().Payments().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), "p-1", "org-1").
		Return(&domain.Payment{ID: "p-1", OrganizationID: "org-1", User: "user-1"}, nil)
	repo.EXPECT().Delete(gomock.Any(), "p-1", "org-1").Return(nil)

	require.NoError(t, svc.DeletePayment(context.Background(), "org-1", "p-1"))
}

// TestHistoryService_RequiresOrganization pins the history read fail-fast and
// the scoped repository arguments for attempts and refunds.
func TestHistoryService_RequiresOrganization(t *testing.T) {
	attempts := &scopeAttemptsRepo{}
	refunds := &scopeRefundsRepo{}
	svc := NewHistoryService(attempts, refunds)

	_, err := svc.ListAttemptsByPayment(context.Background(), "", "p-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
	_, err = svc.GetAttemptByID(context.Background(), "", "a-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
	_, err = svc.ListRefundsByPayment(context.Background(), "", "p-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
	_, err = svc.GetRefundByID(context.Background(), "", "r-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	_, err = svc.GetAttemptByID(context.Background(), "org-1", "a-foreign")
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
	require.Equal(t, []string{"org-1"}, attempts.orgs)

	_, err = svc.GetRefundByID(context.Background(), "org-1", "r-foreign")
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
	require.Equal(t, []string{"org-1"}, refunds.orgs)
}

// TestChannelConfigByID_RequiresOrganization pins the channel config fail-fast
// for both the read and the state transition.
func TestChannelConfigByID_RequiresOrganization(t *testing.T) {
	svc := NewChannelConfigService(newFakeCCRepo(), ccFakeClock{}, nil, platformuuid.V7Generator{})
	ctx := context.Background()

	_, err := svc.GetByID(ctx, "", "cfg-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	_, err = svc.Transition(ctx, "", "cfg-1", domain.ChannelConfigStatusTesting)
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	_, err = svc.Disable(ctx, "", "cfg-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
}

// TestReconByID_RequiresOrganization pins the recon fail-fast and that a
// foreign batch id is rejected before the worker is run.
func TestReconByID_RequiresOrganization(t *testing.T) {
	svc := NewReconService(&tenantReconRepo{}, &recordingReconRunner{}, platformuuid.V7Generator{})
	ctx := context.Background()

	_, err := svc.ListItems(ctx, "", "batch-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	err = svc.ResolveItem(ctx, "", "item-1", "ops")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)

	err = svc.RunManual(ctx, "", "batch-1")
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
}

type tenantReconRepo struct{}

func (tenantReconRepo) Batches(context.Context, string, int, int) ([]*domain.ReconBatch, int64, error) {
	return nil, 0, nil
}
func (tenantReconRepo) GetBatch(context.Context, string) (*domain.ReconBatch, error) {
	return nil, apperrors.NewNotFound("recon batch not found")
}
func (tenantReconRepo) GetBatchForOrganization(context.Context, string, string) (*domain.ReconBatch, error) {
	return nil, apperrors.NewNotFound("recon batch not found")
}
func (tenantReconRepo) CreateBatch(context.Context, *domain.ReconBatch) error { return nil }
func (tenantReconRepo) UpdateBatch(context.Context, *domain.ReconBatch) error { return nil }
func (tenantReconRepo) CreateItem(context.Context, *domain.ReconItem) error   { return nil }
func (tenantReconRepo) ListItems(context.Context, string) ([]*domain.ReconItem, error) {
	return nil, nil
}
func (tenantReconRepo) ListItemsForOrganization(context.Context, string, string) ([]*domain.ReconItem, error) {
	return nil, nil
}
func (tenantReconRepo) ResolveItem(context.Context, string, string) error { return nil }
func (tenantReconRepo) ResolveItemForOrganization(context.Context, string, string, string) error {
	return apperrors.NewNotFound("recon item not found")
}

type recordingReconRunner struct{ runs []string }

func (r *recordingReconRunner) RunManual(_ context.Context, batchID string) error {
	r.runs = append(r.runs, batchID)
	return nil
}

type reconUUIDRepo struct {
	created    []*domain.ReconBatch
	failCreate error
}

func (r *reconUUIDRepo) Batches(context.Context, string, int, int) ([]*domain.ReconBatch, int64, error) {
	return nil, 0, nil
}
func (r *reconUUIDRepo) GetBatch(context.Context, string) (*domain.ReconBatch, error) {
	return nil, errors.New("not found")
}
func (r *reconUUIDRepo) GetBatchForOrganization(context.Context, string, string) (*domain.ReconBatch, error) {
	return nil, errors.New("not found")
}
func (r *reconUUIDRepo) CreateBatch(_ context.Context, b *domain.ReconBatch) error {
	if r.failCreate != nil {
		return r.failCreate
	}
	r.created = append(r.created, b)
	return nil
}
func (r *reconUUIDRepo) UpdateBatch(context.Context, *domain.ReconBatch) error { return nil }
func (r *reconUUIDRepo) CreateItem(context.Context, *domain.ReconItem) error   { return nil }
func (r *reconUUIDRepo) ListItems(context.Context, string) ([]*domain.ReconItem, error) {
	return nil, nil
}
func (r *reconUUIDRepo) ListItemsForOrganization(context.Context, string, string) ([]*domain.ReconItem, error) {
	return nil, nil
}
func (r *reconUUIDRepo) ResolveItem(context.Context, string, string) error { return nil }
func (r *reconUUIDRepo) ResolveItemForOrganization(context.Context, string, string, string) error {
	return nil
}

// TestCreateAndRun_UsesUUIDBatchID pins that pay_recon_batches.id is a random
// UUID and that the one-batch-per-org/channel/date rule is left to the
// uq_pay_recon_batches_org_channel_date unique constraint.
func TestCreateAndRun_UsesUUIDBatchID(t *testing.T) {
	repo := &reconUUIDRepo{}
	runner := &recordingReconRunner{}
	svc := NewReconService(repo, runner, nil)
	ctx := context.Background()
	batchDate := time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)

	firstID, err := svc.CreateAndRun(ctx, "org-1", "alipay", batchDate, now)
	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	require.Len(t, runner.runs, 1)
	assert.True(t, platformuuid.IsValid(firstID), "batch id must be a UUID: %s", firstID)
	assert.Equal(t, firstID, repo.created[0].ID)
	assert.Equal(t, firstID, runner.runs[0])

	// Replaying the same org/channel/date is rejected by the DB unique
	// constraint; the service must surface the error and must not run again.
	repo.failCreate = errors.New("uq_pay_recon_batches_org_channel_date violation")
	_, err = svc.CreateAndRun(ctx, "org-1", "alipay", batchDate, now)
	require.Error(t, err)
	assert.Len(t, runner.runs, 1)

	repo.failCreate = nil
	secondID, err := svc.CreateAndRun(ctx, "org-1", "alipay", batchDate.AddDate(0, 0, 1), now)
	require.NoError(t, err)
	assert.True(t, platformuuid.IsValid(secondID), "batch id must be a UUID: %s", secondID)
	assert.NotEqual(t, firstID, secondID)
}

type scopeAttemptsRepo struct{ orgs []string }

func (r *scopeAttemptsRepo) GetByID(context.Context, string) (*domain.PayAttempt, error) {
	return nil, apperrors.NewNotFound("attempt not found")
}
func (r *scopeAttemptsRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.PayAttempt, error) {
	r.orgs = append(r.orgs, organizationID)
	return nil, apperrors.NewNotFound("attempt not found")
}
func (r *scopeAttemptsRepo) ListByPayment(context.Context, string) ([]*domain.PayAttempt, error) {
	return nil, nil
}
func (r *scopeAttemptsRepo) ListByPaymentForOrganization(_ context.Context, _, organizationID string) ([]*domain.PayAttempt, error) {
	r.orgs = append(r.orgs, organizationID)
	return nil, nil
}
func (r *scopeAttemptsRepo) ListByRefund(context.Context, string) ([]*domain.PayAttempt, error) {
	return nil, nil
}
func (r *scopeAttemptsRepo) Create(context.Context, *domain.PayAttempt) error { return nil }

type scopeRefundsRepo struct{ orgs []string }

func (r *scopeRefundsRepo) GetByID(context.Context, string) (*domain.Refund, error) {
	return nil, apperrors.NewNotFound("refund not found")
}
func (r *scopeRefundsRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Refund, error) {
	r.orgs = append(r.orgs, organizationID)
	return nil, apperrors.NewNotFound("refund not found")
}
func (r *scopeRefundsRepo) GetByOutRefundNo(context.Context, string, string) (*domain.Refund, error) {
	return nil, apperrors.NewNotFound("refund not found")
}
func (r *scopeRefundsRepo) ListByPayment(context.Context, string) ([]*domain.Refund, error) {
	return nil, nil
}
func (r *scopeRefundsRepo) ListByPaymentForOrganization(_ context.Context, _, organizationID string) ([]*domain.Refund, error) {
	r.orgs = append(r.orgs, organizationID)
	return nil, nil
}
func (r *scopeRefundsRepo) List(context.Context, string, int, int) ([]*domain.Refund, int64, error) {
	return nil, 0, nil
}
func (r *scopeRefundsRepo) Create(context.Context, *domain.Refund) error { return nil }
func (r *scopeRefundsRepo) Update(context.Context, *domain.Refund) error { return nil }

func (tenantReconRepo) ApproveItemForOrganization(context.Context, string, string, string) (string, error) {
	return "resolver", nil
}

func (r *reconUUIDRepo) ApproveItemForOrganization(context.Context, string, string, string) (string, error) {
	return "resolver", nil
}
