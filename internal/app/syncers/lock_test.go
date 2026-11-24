package syncers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	syncersmock "github.com/guliangiam/guliangiam/internal/domain/syncers/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// fakeLockProvider records TryLock/release calls for the DB-061 regression tests.
type fakeLockProvider struct {
	acquired      bool
	err           error
	lastName      string
	released      bool
	releaseCtxErr error
}

func (p *fakeLockProvider) TryLock(_ context.Context, name string) (func(context.Context) error, bool, error) {
	p.lastName = name
	if p.err != nil {
		return nil, false, p.err
	}
	if !p.acquired {
		return nil, false, nil
	}
	return func(ctx context.Context) error {
		p.released = true
		p.releaseCtxErr = ctx.Err()
		return nil
	}, true, nil
}

func newLockedTestService(uow appuow.UnitOfWork, engine domain.SyncEngine, locks LockProvider) Service {
	return Service{
		Clock:   fakeClockSync{fixedTime: time.Now()},
		UUID:    fakeUUIDSync{fixedID: "new-id"},
		UOW:     uow,
		Factory: &mockSyncEngineFactoryUOW{engine: engine},
		locks:   locks,
		mu:      &sync.Mutex{},
		running: make(map[string]struct{}),
	}
}

// BE-B-21 regression: when another replica holds the advisory lock, the run is
// rejected with Conflict before any engine work happens.
func TestService_RunSyncer_CrossProcessLockBusy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	syncerRepo := syncersmock.NewMockRepository(ctrl)
	lock := &fakeLockProvider{acquired: false}

	// If the engine were reached, SyncUsers would fail the run with a
	// different error; the Conflict assertion below proves it never ran.
	engine := &mockSyncEngineUOW{userErr: assert.AnError}
	svc := newLockedTestService(mockUOW, engine, lock)

	syncer := &domain.Syncer{ID: "sync1", OrganizationID: "org1", Type: domain.SyncerTypeOkta, IsEnabled: true}
	mockUOW.EXPECT().Syncers().Return(syncerRepo)
	syncerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "sync1", "org1").Return(syncer, nil)

	_, err := svc.RunSyncerForOrganization(context.Background(), "org1", "sync1")
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
	assert.Equal(t, "syncers:syncer:sync1", lock.lastName)
	assert.False(t, lock.released)
	assert.Empty(t, svc.running, "the in-process guard must be unwound on lock rejection")
}

// BE-B-21 regression: a successful run acquires the per-syncer lock and releases
// it afterwards with a live context even when the request context is done.
func TestService_RunSyncer_CrossProcessLockAcquiredAndReleased(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	syncerRepo := syncersmock.NewMockRepository(ctrl)
	lock := &fakeLockProvider{acquired: true}

	engine := &mockSyncEngineUOW{
		userResult:  &domain.SyncEngineResult{AddedUsers: 1},
		groupResult: &domain.SyncEngineResult{AddedGroups: 1},
	}
	svc := newLockedTestService(mockUOW, engine, lock)

	syncer := &domain.Syncer{ID: "sync1", OrganizationID: "org1", Type: domain.SyncerTypeOkta, IsEnabled: true}
	mockUOW.EXPECT().Syncers().Return(syncerRepo).AnyTimes()
	syncerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "sync1", "org1").Return(syncer, nil)
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	syncerRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	reqCtx, cancel := context.WithCancel(context.Background())
	result, err := svc.RunSyncerForOrganization(reqCtx, "org1", "sync1")
	require.NoError(t, err)
	require.NotNil(t, result)
	cancel()

	assert.Equal(t, "syncers:syncer:sync1", lock.lastName)
	assert.True(t, lock.released, "the advisory lock must be released after the run")
	assert.NoError(t, lock.releaseCtxErr, "release must use a fresh context, not the request context")
	assert.Empty(t, svc.running)
}

// BE-B-21: a lock infrastructure failure surfaces as an internal error and
// never starts the engine.
func TestService_RunSyncer_LockProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	syncerRepo := syncersmock.NewMockRepository(ctrl)
	lock := &fakeLockProvider{err: assert.AnError}

	svc := newLockedTestService(mockUOW, &mockSyncEngineUOW{userErr: assert.AnError}, lock)

	syncer := &domain.Syncer{ID: "sync1", OrganizationID: "org1", Type: domain.SyncerTypeOkta, IsEnabled: true}
	mockUOW.EXPECT().Syncers().Return(syncerRepo)
	syncerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "sync1", "org1").Return(syncer, nil)

	_, err := svc.RunSyncerForOrganization(context.Background(), "org1", "sync1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire syncer lock")
	assert.Empty(t, svc.running)
}

func TestService_WithLockProvider_NilKeepsExisting(t *testing.T) {
	lock := &fakeLockProvider{acquired: true}
	svc := Service{locks: lock}
	got := svc.WithLockProvider(nil)
	assert.Equal(t, LockProvider(lock), got.locks)
}
