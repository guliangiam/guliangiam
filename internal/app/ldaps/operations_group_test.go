package ldaps

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	"github.com/guliangiam/guliangiam/internal/domain/groups"
	groupsmock "github.com/guliangiam/guliangiam/internal/domain/groups/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// byNameGroupRepo adds the optional exact-name capability on top of the
// generated domain mock, mirroring the postgres repository.
type byNameGroupRepo struct {
	*groupsmock.MockRepository
	byName func(ctx context.Context, orgID, name string) (*groups.Group, error)
	calls  int
}

func (r *byNameGroupRepo) GetByNameForOrganization(ctx context.Context, orgID, name string) (*groups.Group, error) {
	r.calls++
	return r.byName(ctx, orgID, name)
}

func groupUpsertService(t *testing.T, repo groups.Repository) (Service, *appuowmock.MockUnitOfWork) {
	t.Helper()
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	uow.EXPECT().Groups().Return(repo).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		}).AnyTimes()
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "new-group-id"},
		UOW:   uow,
	}
	return svc, uow
}

// BE-B-14: with the exact-name capability the upsert resolves the group in one
// equality read — no keyword list scan at all (List has no EXPECT).
func TestUpsertLDAPGroup_ExactNameLookupUpdatesExisting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	existing := &groups.Group{ID: "g1", OrganizationID: "org1", Name: "engineers", DisplayName: "Old"}
	repo := &byNameGroupRepo{
		MockRepository: groupsmock.NewMockRepository(ctrl),
		byName: func(_ context.Context, orgID, name string) (*groups.Group, error) {
			assert.Equal(t, "org1", orgID)
			assert.Equal(t, "engineers", name)
			return existing, nil
		},
	}
	svc, _ := groupUpsertService(t, repo)

	var updated *groups.Group
	repo.MockRepository.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, g *groups.Group) error {
			updated = g
			return nil
		})
	// No Create expectation: an exact match must never create a duplicate.

	err := svc.upsertLDAPGroup(context.Background(), "org1", &domain.LDAPMappedGroup{Name: "engineers", DisplayName: "Engineering"})
	require.NoError(t, err)
	assert.Equal(t, 1, repo.calls)
	require.NotNil(t, updated)
	assert.Equal(t, "g1", updated.ID)
	assert.Equal(t, "Engineering", updated.DisplayName)
}

func TestUpsertLDAPGroup_ExactNameNotFoundCreates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := &byNameGroupRepo{
		MockRepository: groupsmock.NewMockRepository(ctrl),
		byName: func(_ context.Context, _, _ string) (*groups.Group, error) {
			return nil, apperrors.NewNotFound("group not found")
		},
	}
	svc, _ := groupUpsertService(t, repo)

	var created *groups.Group
	repo.MockRepository.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, g *groups.Group) error {
			created = g
			return nil
		})

	err := svc.upsertLDAPGroup(context.Background(), "org1", &domain.LDAPMappedGroup{Name: "engineers", DisplayName: "Engineering"})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "engineers", created.Name)
	assert.Equal(t, "org1", created.OrganizationID)
}

// A non-NotFound lookup error must surface, not silently create a duplicate.
func TestUpsertLDAPGroup_ExactNameLookupErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := &byNameGroupRepo{
		MockRepository: groupsmock.NewMockRepository(ctrl),
		byName: func(_ context.Context, _, _ string) (*groups.Group, error) {
			return nil, apperrors.NewInternal("db down")
		},
	}
	svc, _ := groupUpsertService(t, repo)
	// No Create/Update expectations: nothing may be written on a lookup error.

	err := svc.upsertLDAPGroup(context.Background(), "org1", &domain.LDAPMappedGroup{Name: "engineers"})
	require.Error(t, err)
}

// Fallback (mirrors/test doubles without the capability): the keyword scan is
// a substring match, so the exact name must be picked out of many prefix
// matches regardless of position — the old first-page-only scan created
// duplicates when the exact match sat behind other rows.
func TestUpsertLDAPGroup_FallbackScanMatchesExactNameAmongPrefixes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rows := make([]*groups.Group, 0, 12)
	for i := 0; i < 11; i++ {
		rows = append(rows, &groups.Group{ID: "g-prefix", OrganizationID: "org1", Name: "engineers-extra"})
	}
	rows = append(rows, &groups.Group{ID: "g-exact", OrganizationID: "org1", Name: "engineers", DisplayName: "Old"})

	repo := groupsmock.NewMockRepository(ctrl)
	svc, _ := groupUpsertService(t, repo)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, q groups.ListQuery) ([]*groups.Group, int64, error) {
			assert.Equal(t, "engineers", q.Keyword)
			if q.Page > 1 {
				return nil, int64(len(rows)), nil
			}
			return rows, int64(len(rows)), nil
		}).AnyTimes()

	var updated *groups.Group
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, g *groups.Group) error {
			updated = g
			return nil
		})
	// No Create expectation: the exact match exists and must be updated.

	err := svc.upsertLDAPGroup(context.Background(), "org1", &domain.LDAPMappedGroup{Name: "engineers", DisplayName: "Engineering"})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "g-exact", updated.ID)
}
