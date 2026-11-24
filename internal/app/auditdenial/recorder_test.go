package auditdenial

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/time/rate"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	recordsmock "github.com/guliangiam/guliangiam/internal/domain/records/mock"
	platformaudit "github.com/guliangiam/guliangiam/internal/platform/auditdenial"
)

func TestRecorderDeduplicatesWithinWindow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uow := appuowmock.NewMockAuditUoW(ctrl)
	tx := appuowmock.NewMockUnitOfWork(ctrl)
	repo := recordsmock.NewMockRepository(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, tx)
		}).Times(1)
	tx.EXPECT().Records().Return(repo).AnyTimes()
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, record *domainrecords.Record) error {
			assert.Equal(t, "authz.cross_tenant_denied", record.Action)
			assert.Equal(t, 1, record.Metadata["count"])
			return nil
		}).Times(1)

	recorder := &Recorder{
		UOW: uow, Deduper: NewMemoryDeduper(),
		Limiter: rate.NewLimiter(rate.Inf, 1), DedupWin: time.Minute,
	}
	event := platformaudit.Event{OrganizationID: "org-1", ActorID: "user-1", TargetTenant: "org-2", Action: "authz.cross_tenant_denied", Reason: "cross_tenant"}
	recorder.Record(context.Background(), event)
	recorder.Record(context.Background(), event)
}

func TestRecorderIgnoresMissingTenant(t *testing.T) {
	recorder := &Recorder{}
	recorder.Record(context.Background(), platformaudit.Event{})
	require.True(t, true)
}
