package organizations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// pagedOrganizationNameRepository serves one page at a time and clamps the
// requested size like the postgres repository, so an oversized request
// demonstrates the silent truncation the walk replaces.
type pagedOrganizationNameRepository struct {
	orgdomain.Repository
	orgs []*orgdomain.Organization
}

func (r pagedOrganizationNameRepository) List(_ context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	pageSize := platformpagination.ClampPageSize(query.PageSize)
	offset := platformpagination.Offset(query.Page, pageSize)
	if offset >= len(r.orgs) {
		return nil, int64(len(r.orgs)), nil
	}
	end := offset + pageSize
	if end > len(r.orgs) {
		end = len(r.orgs)
	}
	return r.orgs[offset:end], int64(len(r.orgs)), nil
}

type pagedOrganizationNameUoW struct {
	appuow.IdentityUoW
	repo orgdomain.Repository
}

func (u pagedOrganizationNameUoW) Organizations() orgdomain.Repository { return u.repo }

// The name catalog feeds pickers, so it must contain organizations beyond the
// first page: the previous single PageSize:1000 read was clamped to
// MaxPageSize and silently truncated at 100.
func TestService_GetOrganizationNames_ScansBeyondTheFirstPage(t *testing.T) {
	orgs := make([]*orgdomain.Organization, 2*platformpagination.MaxPageSize+1)
	for i := range orgs {
		orgs[i] = &orgdomain.Organization{
			ID:          fmt.Sprintf("org-%03d", i),
			DisplayName: fmt.Sprintf("Org %03d", i),
			Slug:        fmt.Sprintf("org-%03d", i),
		}
	}

	svc := Service{UOW: pagedOrganizationNameUoW{repo: pagedOrganizationNameRepository{orgs: orgs}}}
	items, err := svc.GetOrganizationNames(context.Background())
	require.NoError(t, err)
	require.Len(t, items, len(orgs))
	assert.Equal(t, "org-200", items[len(items)-1].ID)
}
