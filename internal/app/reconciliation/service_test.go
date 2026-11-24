package reconciliation

import (
	"context"
	"math"
	"testing"
	"time"

	scimdomain "github.com/guliangiam/guliangiam/internal/domain/scim"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeReportRepo struct {
	reports []*scimdomain.ReconciliationReport
	limit   int
	offset  int
}

func (f *fakeReportRepo) Create(context.Context, *scimdomain.ReconciliationReport) error { return nil }

func (f *fakeReportRepo) ListByOrganization(_ context.Context, _ string, limit, offset int) ([]*scimdomain.ReconciliationReport, int64, error) {
	f.limit, f.offset = limit, offset
	return f.reports, int64(len(f.reports)), nil
}

func TestServiceList_MapsAndClampsPagination(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeReportRepo{reports: []*scimdomain.ReconciliationReport{{
		ID: "r-1", OrganizationID: "org-1", ConnectorID: "c-1", Status: "completed",
		StartedAt: now, FinishedAt: now, UserReport: []byte(`{}`), GroupReport: []byte(`{}`), CreatedAt: now,
	}}}
	response, err := NewService(repo).List(context.Background(), ListQuery{
		OrganizationID: "org-1", Page: 2, PageSize: 500,
	})
	require.NoError(t, err)
	require.Len(t, response.Items, 1)
	assert.Equal(t, "r-1", response.Items[0].ID)
	assert.Equal(t, 100, response.Page.PageSize, "page size clamps to the shared maximum")
	assert.Equal(t, 100, repo.limit)
	assert.Equal(t, 100, repo.offset)
	assert.Equal(t, 2, response.Page.Page)
	assert.Equal(t, int64(1), response.Page.Total)
}

func TestServiceList_HostilePaginationKeepsOffsetNonNegative(t *testing.T) {
	cases := []struct {
		name               string
		page, pageSize     int
		wantLimit, wantOff int
	}{
		{
			name: "hostile page is capped",
			page: math.MaxInt, pageSize: 20,
			wantLimit: 20, wantOff: (platformpagination.MaxPage - 1) * 20,
		},
		{
			name: "hostile page size is capped",
			page: 2, pageSize: math.MaxInt,
			wantLimit: platformpagination.MaxPageSize, wantOff: platformpagination.MaxPageSize,
		},
		{
			name: "normal page",
			page: 3, pageSize: 25,
			wantLimit: 25, wantOff: 50,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeReportRepo{}
			response, err := NewService(repo).List(context.Background(), ListQuery{
				OrganizationID: "org-1", Page: tc.page, PageSize: tc.pageSize,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.wantLimit, repo.limit)
			assert.Equal(t, tc.wantOff, repo.offset)
			assert.GreaterOrEqual(t, repo.offset, 0)
			assert.Equal(t, tc.wantLimit, response.Page.PageSize)
		})
	}
}

func TestServiceList_UnavailableWithoutRepository(t *testing.T) {
	_, err := NewService(nil).List(context.Background(), ListQuery{OrganizationID: "org-1"})
	require.Error(t, err)
}
