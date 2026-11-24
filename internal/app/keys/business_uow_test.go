package keys

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	repoMock "github.com/guliangiam/guliangiam/internal/domain/keys/mock"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	keyTestOrgID      = "11111111-1111-1111-1111-111111111111"
	keyTestOtherOrgID = "22222222-2222-2222-2222-222222222222"
)

func TestService_GetEnabledKeyByName_EmptyName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	svc := Service{UOW: mockUOW}

	_, err := svc.GetEnabledKeyByName(context.Background(), keyTestOrgID, "")
	assert.Error(t, err)
}

func TestService_GetEnabledKeyByName_ListError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: mockUOW}

	mockUOW.EXPECT().Keys().Return(mockKeysRepo)
	mockKeysRepo.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), assert.AnError)

	_, err := svc.GetEnabledKeyByName(context.Background(), keyTestOrgID, "test-key")
	assert.Error(t, err)
}

func TestService_GetEnabledKeyByName_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: mockUOW}

	mockUOW.EXPECT().Keys().Return(mockKeysRepo)
	enabled := true
	mockKeysRepo.EXPECT().List(gomock.Any(), domain.ListQuery{
		Page: 1, PageSize: 100, OrganizationID: keyTestOrgID, Keyword: "test-key", IsEnabled: &enabled,
	}).Return([]*domain.Key{}, int64(0), nil)

	_, err := svc.GetEnabledKeyByName(context.Background(), keyTestOrgID, "test-key")
	assert.Error(t, err)
}

func TestService_GetEnabledKeyByName_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: mockUOW}

	enabled := true
	k := &domain.Key{ID: "key1", Name: "test-key", IsEnabled: true, OrganizationID: keyTestOrgID}
	mockUOW.EXPECT().Keys().Return(mockKeysRepo)
	mockKeysRepo.EXPECT().List(gomock.Any(), domain.ListQuery{
		Page: 1, PageSize: 100, OrganizationID: keyTestOrgID, Keyword: "test-key", IsEnabled: &enabled,
	}).Return([]*domain.Key{k}, int64(1), nil)

	result, err := svc.GetEnabledKeyByName(context.Background(), keyTestOrgID, "test-key")
	require.NoError(t, err)
	assert.Equal(t, "key1", result.ID)
}

func TestService_GetEnabledKeyByName_EmptyOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	svc := Service{UOW: mockUOW}

	_, err := svc.GetEnabledKeyByName(context.Background(), "  ", "app-signing-app1")
	assert.Error(t, err)
}

func TestService_GetEnabledKeyByName_ExactNameWinsOverPrefix(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: mockUOW}

	enabled := true
	mockUOW.EXPECT().Keys().Return(mockKeysRepo)
	mockKeysRepo.EXPECT().List(gomock.Any(), domain.ListQuery{
		Page: 1, PageSize: 100, OrganizationID: keyTestOrgID, Keyword: "app-signing-app1", IsEnabled: &enabled,
	}).Return([]*domain.Key{
		{ID: "prefix", Name: "app-signing-app10", OrganizationID: keyTestOrgID},
		{ID: "exact", Name: "app-signing-app1", OrganizationID: keyTestOrgID},
	}, int64(2), nil)

	result, err := svc.GetEnabledKeyByName(context.Background(), keyTestOrgID, "app-signing-app1")
	require.NoError(t, err)
	assert.Equal(t, "exact", result.ID)
}

func TestService_GetEnabledKeyByName_SameNameInOtherTenantIsNotReturned(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: mockUOW}

	enabled := true
	mockUOW.EXPECT().Keys().Return(mockKeysRepo)
	// The repository is asked for one organization only. A mirror that ignores
	// that predicate and returns the other tenant's same-name row must still not
	// produce a key for the requested organization.
	mockKeysRepo.EXPECT().List(gomock.Any(), domain.ListQuery{
		Page: 1, PageSize: 100, OrganizationID: keyTestOrgID, Keyword: "app-signing-app1", IsEnabled: &enabled,
	}).Return([]*domain.Key{
		{ID: "foreign", Name: "app-signing-app1", OrganizationID: keyTestOtherOrgID},
	}, int64(1), nil)

	_, err := svc.GetEnabledKeyByName(context.Background(), keyTestOrgID, "app-signing-app1")
	assert.Error(t, err)
}

func TestService_DisableKey_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: mockUOW, Clock: fakeClock{fixedTime: time.Now()}}

	k := &domain.Key{ID: "key1", OrganizationID: keyTestOrgID, Revision: 1, IsEnabled: true}

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockKeysRepo.EXPECT().GetByID(gomock.Any(), "key1", keyTestOrgID).Return(k, nil)
	mockKeysRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	err := svc.DisableKey(context.Background(), keyTestOrgID, "key1", 1)
	require.NoError(t, err)
	assert.False(t, k.IsEnabled)
}

func TestService_DisableKey_RevisionMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: mockUOW, Clock: fakeClock{fixedTime: time.Now()}}

	k := &domain.Key{ID: "key1", OrganizationID: keyTestOrgID, Revision: 2, IsEnabled: true}

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockKeysRepo.EXPECT().GetByID(gomock.Any(), "key1", keyTestOrgID).Return(k, nil)

	err := svc.DisableKey(context.Background(), keyTestOrgID, "key1", 1)
	assert.Error(t, err)
}

func TestService_PatchKey_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)
	svc := Service{UOW: mockUOW, Clock: fakeClock{fixedTime: time.Now()}}

	k := &domain.Key{ID: "key1", OrganizationID: keyTestOrgID, Revision: 1}
	displayName := "Updated Name"

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockKeysRepo.EXPECT().GetByID(gomock.Any(), "key1", keyTestOrgID).Return(k, nil)
	mockKeysRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	err := svc.PatchKey(context.Background(), keyTestOrgID, PatchKeyCommand{
		OrganizationID: keyTestOrgID,
		ID:             "key1",
		DisplayName:    &displayName,
		Revision:       1,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", k.DisplayName)
}

func TestService_DeleteKey_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)

	records := &capturingRecordsRepo{}
	outbox := &capturingOutboxRepo{}
	tx := &auditCaptureUoW{MockUnitOfWork: mockUOW, records: records, outbox: outbox}
	svc := Service{UOW: mockUOW, Clock: fakeClock{fixedTime: time.Now()}}

	k := &domain.Key{ID: "key1", OrganizationID: keyTestOrgID, Name: "signing-key", Revision: 1}

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, tx)
		},
	)
	mockKeysRepo.EXPECT().GetByID(gomock.Any(), "key1", keyTestOrgID).Return(k, nil)
	mockKeysRepo.EXPECT().Delete(gomock.Any(), "key1", keyTestOrgID).Return(nil)

	err := svc.DeleteKey(context.Background(), keyTestOrgID, "key1")
	require.NoError(t, err)

	require.Len(t, records.created, 1)
	assert.Equal(t, keyTestOrgID, records.created[0].OrganizationID)
	assert.Equal(t, "key.deleted", records.created[0].Action)
	assert.Equal(t, "key1", records.created[0].ResourceID)

	require.Len(t, outbox.events, 1)
	require.NotNil(t, outbox.events[0].OrganizationID)
	assert.Equal(t, keyTestOrgID, *outbox.events[0].OrganizationID)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(outbox.events[0].Payload, &payload))
	assert.Equal(t, keyTestOrgID, payload["orgId"])
	assert.Equal(t, "key1", payload["keyId"])
	assert.Equal(t, "signing-key", payload["name"])
}

// TestService_DeleteKey_CrossTenantNotFound pins that a delete for another
// tenant's key stops at the tenant-scoped lookup: no row is removed and no
// audit event is embedded.
func TestService_DeleteKey_CrossTenantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)
	records := &capturingRecordsRepo{}
	outbox := &capturingOutboxRepo{}
	tx := &auditCaptureUoW{MockUnitOfWork: mockUOW, records: records, outbox: outbox}
	svc := Service{UOW: mockUOW, Clock: fakeClock{fixedTime: time.Now()}}

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, tx)
		},
	)
	mockKeysRepo.EXPECT().GetByID(gomock.Any(), "foreign-key", keyTestOrgID).
		Return(nil, apperrors.NewNotFound("key not found"))

	err := svc.DeleteKey(context.Background(), keyTestOrgID, "foreign-key")
	require.Error(t, err)
	assert.Empty(t, records.created)
	assert.Empty(t, outbox.events)
}

// TestService_DeleteKey_AuditUsesEntityOrganization pins that the delete audit
// row and outbox event carry the organization read from the entity, not the
// caller-supplied scope argument.
func TestService_DeleteKey_AuditUsesEntityOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)
	records := &capturingRecordsRepo{}
	outbox := &capturingOutboxRepo{}
	tx := &auditCaptureUoW{MockUnitOfWork: mockUOW, records: records, outbox: outbox}
	svc := Service{UOW: mockUOW, Clock: fakeClock{fixedTime: time.Now()}}

	// The scoped read returns a row owned by a different organization: the
	// audit must record the entity's real owner.
	entity := &domain.Key{ID: "key1", OrganizationID: keyTestOtherOrgID, Name: "signing-key", Revision: 1}

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, tx)
		},
	)
	mockKeysRepo.EXPECT().GetByID(gomock.Any(), "key1", keyTestOrgID).Return(entity, nil)
	mockKeysRepo.EXPECT().Delete(gomock.Any(), "key1", keyTestOrgID).Return(nil)

	err := svc.DeleteKey(context.Background(), keyTestOrgID, "key1")
	require.NoError(t, err)

	require.Len(t, records.created, 1)
	assert.Equal(t, keyTestOtherOrgID, records.created[0].OrganizationID)
	require.Len(t, outbox.events, 1)
	require.NotNil(t, outbox.events[0].OrganizationID)
	assert.Equal(t, keyTestOtherOrgID, *outbox.events[0].OrganizationID)
}

func TestService_CreateKey_CustomAlgorithm(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUOW := appuowmock.NewMockUnitOfWork(ctrl)
	mockKeysRepo := repoMock.NewMockRepository(ctrl)

	mockUOW.EXPECT().Keys().Return(mockKeysRepo).AnyTimes()
	mockUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, mockUOW)
		},
	)
	mockKeysRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, k *domain.Key) error {
			assert.Equal(t, "ES256", k.Algorithm)
			return nil
		},
	)

	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "k-001"},
		UOW:   mockUOW,
	}

	err := svc.CreateKey(context.Background(), CreateKeyCommand{
		OrganizationID: "org-1",
		Name:           "test-key",
		KeyType:        "EC",
		Algorithm:      "ES256",
	})
	require.NoError(t, err)
}

// auditCaptureUoW wraps the gomock UnitOfWork so audit embedding is not
// short-circuited by the generated mock's isgomock marker. Records() and
// Outbox() are overridden with capturing doubles; every other repository is
// still served by the embedded mock.
type auditCaptureUoW struct {
	*appuowmock.MockUnitOfWork
	records *capturingRecordsRepo
	outbox  *capturingOutboxRepo
}

func (u *auditCaptureUoW) Records() domainrecords.Repository { return u.records }

func (u *auditCaptureUoW) Outbox() domainoutbox.Repository { return u.outbox }

type capturingRecordsRepo struct {
	created []*domainrecords.Record
}

func (r *capturingRecordsRepo) Create(_ context.Context, record *domainrecords.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *capturingRecordsRepo) List(context.Context, domainrecords.ListQuery) ([]*domainrecords.Record, int64, error) {
	return nil, 0, nil
}

func (r *capturingRecordsRepo) Search(context.Context, domainrecords.SearchCriteria) ([]*domainrecords.Record, int64, error) {
	return nil, 0, nil
}

func (r *capturingRecordsRepo) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *capturingRecordsRepo) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *capturingRecordsRepo) InsertAnchor(context.Context, *domainrecords.ChainAnchor) error {
	return nil
}

func (r *capturingRecordsRepo) ListChainEntries(context.Context, string) ([]domainrecords.ChainEntry, error) {
	return nil, nil
}

type capturingOutboxRepo struct {
	events []*domainoutbox.OutboxEvent
}

func (r *capturingOutboxRepo) Enqueue(_ context.Context, item *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, item)
	return nil
}

func (r *capturingOutboxRepo) EnqueueBatch(_ context.Context, items []*domainoutbox.OutboxEvent) error {
	r.events = append(r.events, items...)
	return nil
}
