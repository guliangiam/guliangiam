package applications

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// pagedNameApplicationRepository serves one page at a time and reports the
// unpaged total, like the postgres repository.
type pagedNameApplicationRepository struct {
	domain.Repository
	apps []*domain.Application
}

func (r pagedNameApplicationRepository) List(_ context.Context, query domain.ListQuery) ([]*domain.Application, int64, error) {
	offset := platformpagination.Offset(query.Page, query.PageSize)
	if offset >= len(r.apps) {
		return nil, int64(len(r.apps)), nil
	}
	end := offset + query.PageSize
	if end > len(r.apps) {
		end = len(r.apps)
	}
	return r.apps[offset:end], int64(len(r.apps)), nil
}

type pagedNameApplicationUoW struct {
	appuow.UnitOfWork
	repo domain.Repository
}

func (u pagedNameApplicationUoW) Applications() domain.Repository { return u.repo }

// The name catalog is consumed by pickers, so it must contain applications
// beyond the first page: the previous single PageSize:1000 read was clamped to
// MaxPageSize and silently truncated at 100.
func TestService_GetApplicationNames_ScansBeyondTheFirstPage(t *testing.T) {
	apps := make([]*domain.Application, 2*platformpagination.MaxPageSize+1)
	for i := range apps {
		apps[i] = &domain.Application{
			ID:          fmt.Sprintf("app-%03d", i),
			Slug:        fmt.Sprintf("app-%03d", i),
			DisplayName: fmt.Sprintf("App %03d", i),
		}
	}

	svc := Service{UOW: pagedNameApplicationUoW{repo: pagedNameApplicationRepository{apps: apps}}}
	items, err := svc.GetApplicationNames(context.Background())
	require.NoError(t, err)
	require.Len(t, items, len(apps))
	assert.Equal(t, "app-200", items[len(items)-1].ID)
}
