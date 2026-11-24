package gateway

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	gwmock "github.com/guliangiam/guliangiam/internal/domain/gateway/mock"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// The global site view must include every site: the previous PageSize:1000
// query was clamped to MaxPageSize, so sites beyond the first 100 were invisible.
func TestService_GetGlobalSites_ScansBeyondTheFirstPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := gwmock.NewMockRepository(ctrl)
	uow.EXPECT().Gateway().Return(repo).AnyTimes()

	sites := make([]*domain.Site, 2*platformpagination.MaxPageSize+1)
	for i := range sites {
		sites[i] = &domain.Site{ID: fmt.Sprintf("site-%03d", i), OrganizationID: "org-1", Name: fmt.Sprintf("site-%03d", i)}
	}
	total := int64(len(sites))
	repo.EXPECT().ListSites(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListSitesQuery) ([]*domain.Site, int64, error) {
			assert.Equal(t, platformpagination.MaxPageSize, query.PageSize)
			offset := platformpagination.Offset(query.Page, query.PageSize)
			if offset >= len(sites) {
				return nil, total, nil
			}
			end := offset + query.PageSize
			if end > len(sites) {
				end = len(sites)
			}
			return sites[offset:end], total, nil
		}).Times(3)

	svc := Service{UOW: uow}
	items, err := svc.GetGlobalSites(context.Background())
	require.NoError(t, err)
	require.Len(t, items, len(sites))
	assert.Equal(t, "site-200", items[len(items)-1].ID)
}
