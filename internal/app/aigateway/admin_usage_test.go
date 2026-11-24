package aigateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

type recordingUsageLogRepo struct {
	limit  int
	offset int
}

func (r *recordingUsageLogRepo) Create(context.Context, *domain.UsageLog) error { return nil }

func (r *recordingUsageLogRepo) List(_ context.Context, _ string, limit, offset int) ([]*domain.UsageLog, int64, error) {
	r.limit, r.offset = limit, offset
	return nil, 0, nil
}

func (r *recordingUsageLogRepo) QueryStats(context.Context, string, int) (*domain.UsageStats, error) {
	return &domain.UsageStats{}, nil
}

// The service must not keep its own 50/100 pair: an omitted limit is served as
// the platform default and an oversized one is clamped to the served ceiling.
func TestAdminService_ListUsageLogs_UsesPlatformPageBounds(t *testing.T) {
	t.Parallel()
	repo := &recordingUsageLogRepo{}
	svc := &AdminService{UsageLogs: repo}

	_, _, err := svc.ListUsageLogs(context.Background(), "org-1", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, platformpagination.DefaultPageSize, repo.limit)

	_, _, err = svc.ListUsageLogs(context.Background(), "org-1", platformpagination.MaxPageSize+1, 0)
	require.NoError(t, err)
	assert.Equal(t, platformpagination.MaxPageSize, repo.limit)
}
