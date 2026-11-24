package adapters

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	adaptersmock "github.com/guliangiam/guliangiam/internal/domain/adapters/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestByIDOperations_RequireOrganizationID pins the app-layer fail-fast: an
// empty trusted tenant never reaches the repository (MySQL has no RLS).
func TestByIDOperations_RequireOrganizationID(t *testing.T) {
	t.Parallel()
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "test"}}
	ctx := context.Background()

	cases := map[string]func() error{
		"get": func() error {
			_, err := svc.GetAdapter(ctx, "   ", "a1")
			return err
		},
		"patch": func() error {
			return svc.PatchAdapter(ctx, "", PatchAdapterCommand{ID: "a1"})
		},
		"delete": func() error {
			return svc.DeleteAdapter(ctx, "", "a1")
		},
		"test-connection": func() error {
			_, err := svc.TestConnection(ctx, "", "a1", TestConnectionCommand{})
			return err
		},
		"get-policies": func() error {
			_, err := svc.GetPolicies(ctx, "", "a1")
			return err
		},
		"get-filtered-policies": func() error {
			_, err := svc.GetFilteredPolicies(ctx, "", "a1", 0, "alice")
			return err
		},
		"add-policy": func() error {
			return svc.AddPolicy(ctx, "", "a1", []string{"p", "alice"})
		},
		"remove-policy": func() error {
			return svc.RemovePolicy(ctx, "", "a1", []string{"p", "alice"})
		},
		"update-policy": func() error {
			return svc.UpdatePolicy(ctx, "", "a1", []string{"p", "alice"}, []string{"p", "bob"})
		},
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			err := call()
			require.Error(t, err)
			appErr := apperrors.FromError(err)
			assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
			assert.Contains(t, appErr.Message, "organizationId is required")
		})
	}
}

// TestGetAdapter_CrossTenantIsNotFound pins that a scoped lookup maps a foreign
// adapter to not-found, the same response as a missing id.
func TestGetAdapter_CrossTenantIsNotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	repo.EXPECT().GetByID(gomock.Any(), "foreign-adapter", "own-org").
		Return(nil, apperrors.NewNotFound("adapter not found"))

	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	_, err := svc.GetAdapter(context.Background(), "own-org", "foreign-adapter")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

// TestPatchAdapter_CrossTenantWritesNothing pins that ownership is decided
// before any mutation: a foreign id never reaches Update.
func TestPatchAdapter_CrossTenantWritesNothing(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "foreign-adapter", "own-org").
		Return(nil, apperrors.NewNotFound("adapter not found"))
	// No Update expectation: a foreign id must never be written.

	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	err := svc.PatchAdapter(context.Background(), "own-org", PatchAdapterCommand{ID: "foreign-adapter"})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

// TestDeleteAdapter_CrossTenantWritesNothing pins that a foreign id never
// reaches Delete and therefore produces no audit record.
func TestDeleteAdapter_CrossTenantWritesNothing(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	)
	repo.EXPECT().GetByID(gomock.Any(), "foreign-adapter", "own-org").
		Return(nil, apperrors.NewNotFound("adapter not found"))
	// No Delete expectation: a foreign id must never be removed and no audit
	// write (embedded after Delete) can happen.

	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	err := svc.DeleteAdapter(context.Background(), "own-org", "foreign-adapter")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

// TestTestConnection_CrossTenantIsNotFound pins that stored credentials of a
// foreign adapter are never used for an outbound connection test.
func TestTestConnection_CrossTenantIsNotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	repo.EXPECT().GetByID(gomock.Any(), "foreign-adapter", "own-org").
		Return(nil, apperrors.NewNotFound("adapter not found"))

	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	_, err := svc.TestConnection(context.Background(), "own-org", "foreign-adapter", TestConnectionCommand{})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
}

// TestPolicyOperations_CrossTenantAreNotFound pins that policy reads and writes
// are gated by the same scoped adapter lookup.
func TestPolicyOperations_CrossTenantAreNotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := adaptersmock.NewMockRepository(ctrl)
	uow.EXPECT().Adapters().Return(repo).AnyTimes()
	repo.EXPECT().GetByID(gomock.Any(), "foreign-adapter", "own-org").
		Return(nil, apperrors.NewNotFound("adapter not found")).AnyTimes()

	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "test"}, UOW: uow}
	calls := map[string]func() error{
		"get": func() error {
			_, err := svc.GetPolicies(context.Background(), "own-org", "foreign-adapter")
			return err
		},
		"add": func() error {
			return svc.AddPolicy(context.Background(), "own-org", "foreign-adapter", []string{"p", "alice"})
		},
		"remove": func() error {
			return svc.RemovePolicy(context.Background(), "own-org", "foreign-adapter", []string{"p", "alice"})
		},
		"update": func() error {
			return svc.UpdatePolicy(context.Background(), "own-org", "foreign-adapter", []string{"p", "alice"}, []string{"p", "bob"})
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			require.Error(t, err)
			assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
		})
	}
}
