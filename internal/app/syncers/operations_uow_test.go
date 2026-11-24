package syncers

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	idvdomain "github.com/guliangiam/guliangiam/internal/domain/idv"
	rebac "github.com/guliangiam/guliangiam/internal/domain/rebac"
	riskdomain "github.com/guliangiam/guliangiam/internal/domain/risk"
	domain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	syncersmock "github.com/guliangiam/guliangiam/internal/domain/syncers/mock"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	usersmock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// mockSyncEngineFactory implements domain.SyncEngineFactory
type mockSyncEngineFactoryUOW struct {
	engine domain.SyncEngine
	err    error
}

func (u mockSyncEngineFactoryUOW) IDV() idvdomain.Repository { return nil }

func (u mockSyncEngineFactoryUOW) Risk() riskdomain.Repository { return nil }

func (u mockSyncEngineFactoryUOW) Rebac() rebac.Repository { return nil }

func (m *mockSyncEngineFactoryUOW) GetEngine(providerType string) (domain.SyncEngine, error) {
	return m.engine, m.err
}

// mockSyncEngineUOW implements domain.SyncEngine
type mockSyncEngineUOW struct {
	userResult  *domain.SyncEngineResult
	userErr     error
	groupResult *domain.SyncEngineResult
	groupErr    error
	testErr     error
}

func (u mockSyncEngineUOW) IDV() idvdomain.Repository { return nil }

func (u mockSyncEngineUOW) Risk() riskdomain.Repository { return nil }

func (u mockSyncEngineUOW) Rebac() rebac.Repository { return nil }

func (m *mockSyncEngineUOW) SyncUsers(ctx context.Context, config *domain.SyncEngineConfig) (*domain.SyncEngineResult, error) {
	return m.userResult, m.userErr
}

func (m *mockSyncEngineUOW) SyncGroups(ctx context.Context, config *domain.SyncEngineConfig) (*domain.SyncEngineResult, error) {
	return m.groupResult, m.groupErr
}

func (m *mockSyncEngineUOW) TestConnection(ctx context.Context, config *domain.SyncEngineConfig) error {
	return m.testErr
}

func TestService_TestConnection_WithFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	syncerRepo := syncersmock.NewMockRepository(ctrl)

	svc := Service{
		Clock:   clock.SystemClock{},
		UOW:     mockUOW,
		Factory: &mockSyncEngineFactoryUOW{engine: &mockSyncEngineUOW{testErr: nil}, err: nil},
		mu:      &sync.Mutex{},
		running: make(map[string]struct{}),
	}

	syncer := &domain.Syncer{ID: "sync1", OrganizationID: "org1", Type: domain.SyncerTypeOkta, IsEnabled: true}
	mockUOW.EXPECT().Syncers().Return(syncerRepo)
	syncerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "sync1", "org1").Return(syncer, nil)

	err := svc.TestConnectionForOrganization(context.Background(), "org1", "sync1")
	require.NoError(t, err)
}

func TestService_TestConnection_FactoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	syncerRepo := syncersmock.NewMockRepository(ctrl)

	svc := Service{
		Clock:   clock.SystemClock{},
		UOW:     mockUOW,
		Factory: &mockSyncEngineFactoryUOW{err: assert.AnError},
		mu:      &sync.Mutex{},
		running: make(map[string]struct{}),
	}

	syncer := &domain.Syncer{ID: "sync1", OrganizationID: "org1", Type: domain.SyncerTypeOkta}
	mockUOW.EXPECT().Syncers().Return(syncerRepo)
	syncerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "sync1", "org1").Return(syncer, nil)

	err := svc.TestConnectionForOrganization(context.Background(), "org1", "sync1")
	assert.Error(t, err)
}

func TestService_RunSyncer_AlreadyRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	syncerRepo := syncersmock.NewMockRepository(ctrl)

	svc := Service{
		Clock:   clock.SystemClock{},
		UOW:     mockUOW,
		Factory: &mockSyncEngineFactoryUOW{},
		mu:      &sync.Mutex{},
		running: map[string]struct{}{"sync1": {}},
	}

	syncer := &domain.Syncer{ID: "sync1", OrganizationID: "org1", Type: domain.SyncerTypeOkta, IsEnabled: true}
	mockUOW.EXPECT().Syncers().Return(syncerRepo)
	syncerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "sync1", "org1").Return(syncer, nil)

	_, err := svc.RunSyncerForOrganization(context.Background(), "org1", "sync1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestService_RunSyncer_DisabledSyncer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	syncerRepo := syncersmock.NewMockRepository(ctrl)

	svc := Service{
		Clock:   clock.SystemClock{},
		UOW:     mockUOW,
		Factory: &mockSyncEngineFactoryUOW{},
		mu:      &sync.Mutex{},
		running: make(map[string]struct{}),
	}

	syncer := &domain.Syncer{ID: "sync1", OrganizationID: "org1", Type: domain.SyncerTypeOkta, IsEnabled: false}
	mockUOW.EXPECT().Syncers().Return(syncerRepo)
	syncerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "sync1", "org1").Return(syncer, nil)

	_, err := svc.RunSyncerForOrganization(context.Background(), "org1", "sync1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestService_RunSyncer_FactoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	syncerRepo := syncersmock.NewMockRepository(ctrl)

	svc := Service{
		Clock:   clock.SystemClock{},
		UOW:     mockUOW,
		Factory: &mockSyncEngineFactoryUOW{err: assert.AnError},
		mu:      &sync.Mutex{},
		running: make(map[string]struct{}),
	}

	syncer := &domain.Syncer{ID: "sync1", OrganizationID: "org1", Type: domain.SyncerTypeOkta, IsEnabled: true}
	mockUOW.EXPECT().Syncers().Return(syncerRepo)
	syncerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "sync1", "org1").Return(syncer, nil)

	_, err := svc.RunSyncerForOrganization(context.Background(), "org1", "sync1")
	assert.Error(t, err)
}

func TestService_RunSyncer_SyncUsersError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	syncerRepo := syncersmock.NewMockRepository(ctrl)

	engine := &mockSyncEngineUOW{userErr: assert.AnError}
	svc := Service{
		Clock:   clock.SystemClock{},
		UOW:     mockUOW,
		Factory: &mockSyncEngineFactoryUOW{engine: engine},
		mu:      &sync.Mutex{},
		running: make(map[string]struct{}),
	}

	syncer := &domain.Syncer{ID: "sync1", Type: domain.SyncerTypeOkta, IsEnabled: true, OrganizationID: "org1"}
	mockUOW.EXPECT().Syncers().Return(syncerRepo)
	syncerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "sync1", "org1").Return(syncer, nil)

	_, err := svc.RunSyncerForOrganization(context.Background(), "org1", "sync1")
	assert.Error(t, err)
}

func TestService_RunSyncer_GroupSyncError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	syncerRepo := syncersmock.NewMockRepository(ctrl)

	engine := &mockSyncEngineUOW{
		userResult: &domain.SyncEngineResult{AddedUsers: 1},
		groupErr:   assert.AnError,
	}
	svc := Service{
		Clock:   fakeClockSync{fixedTime: time.Now()},
		UUID:    fakeUUIDSync{fixedID: "new-id"},
		UOW:     mockUOW,
		Factory: &mockSyncEngineFactoryUOW{engine: engine},
		mu:      &sync.Mutex{},
		running: make(map[string]struct{}),
	}

	syncer := &domain.Syncer{ID: "sync1", Type: domain.SyncerTypeOkta, IsEnabled: true, OrganizationID: "org1"}
	mockUOW.EXPECT().Syncers().Return(syncerRepo).AnyTimes()
	syncerRepo.EXPECT().GetByIDForOrganization(gomock.Any(), "sync1", "org1").Return(syncer, nil)
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	syncerRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	result, err := svc.RunSyncerForOrganization(context.Background(), "org1", "sync1")
	require.NoError(t, err)
	assert.NotNil(t, result.UserSync)
	assert.NotNil(t, result.GroupSync)
}

// Renamed fakes to avoid collision with service_test.go
type fakeClockSync struct{ fixedTime time.Time }

func (f fakeClockSync) Now() time.Time { return f.fixedTime }

type fakeUUIDSync struct {
	fixedID string
	err     error
}

func (f fakeUUIDSync) NewString() (string, error) { return f.fixedID, f.err }

type sequenceUUIDGenerator struct{ next int }

func (g *sequenceUUIDGenerator) NewString() (string, error) {
	g.next++
	return fmt.Sprintf("01900000-0000-7000-8000-%012d", g.next), nil
}

// TestUserWriteBack_CreatesUUIDIdentity pins that the syncer-created user and
// its identity get independent UUID primary keys. Business uniqueness stays on
// uq_identity_provider_subject, so no id prefix is part of the contract.
func TestUserWriteBack_CreatesUUIDIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uowMock := appuowmock.NewMockUnitOfWork(ctrl)
	usersRepo := usersmock.NewMockRepository(ctrl)
	svc := Service{
		Clock:   clock.SystemClock{},
		UUID:    &sequenceUUIDGenerator{},
		UOW:     uowMock,
		mu:      &sync.Mutex{},
		running: make(map[string]struct{}),
	}

	var created *users.User
	uowMock.EXPECT().Users().Return(usersRepo).AnyTimes()
	// Only NotFound may fall through to the create branch (BE-B-10); a
	// generic lookup error must surface as a sync failure instead.
	usersRepo.EXPECT().GetByProviderIdentity(gomock.Any(), "okta", "ext-1").
		Return(nil, apperrors.NewNotFound("identity not found"))
	usersRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "alice").
		Return(nil, apperrors.NewNotFound("user not found"))
	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uowMock)
		})
	usersRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, user *users.User) error {
			created = user
			return nil
		})

	createdNow, err := svc.createUserWriteBack("org-1")(
		context.Background(), "okta", "ext-1",
		map[string]interface{}{"username": "alice", "displayName": "Alice"},
	)
	require.NoError(t, err)
	assert.True(t, createdNow)
	require.NotNil(t, created)
	require.Len(t, created.Identities, 1)

	identity := created.Identities[0]
	assert.True(t, platformuuid.IsValid(created.ID), "user id must be a UUID: %s", created.ID)
	assert.True(t, platformuuid.IsValid(identity.ID), "identity id must be a UUID: %s", identity.ID)
	assert.NotEqual(t, created.ID, identity.ID, "user and identity ids must be distinct")
	assert.Equal(t, created.ID, identity.UserID)
	assert.Equal(t, "okta", identity.ProviderID)
	assert.Equal(t, "external", identity.IdentityType)
	assert.Equal(t, "ext-1", identity.Subject)
}
