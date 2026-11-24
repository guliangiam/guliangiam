package ldaps

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	ldapsmock "github.com/guliangiam/guliangiam/internal/domain/ldaps/mock"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

type recordingSyncScheduler struct {
	ids []string
}

func (s *recordingSyncScheduler) AddDomainJob(id string, _ *domain.SyncerConfig, _ func(context.Context, *domain.SyncerConfig) error) error {
	s.ids = append(s.ids, id)
	return nil
}

func (s *recordingSyncScheduler) RemoveJob(string) {}

// Auto-sync scheduling must see every enabled config: the previous
// PageSize:1000 read was clamped to MaxPageSize, so configs beyond the first
// page were never scheduled.
func TestService_ScheduleAllEnabledAutoSyncs_ScansBeyondTheFirstPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	uow := uowmock.NewMockUnitOfWork(ctrl)
	repo := ldapsmock.NewMockRepository(ctrl)
	uow.EXPECT().Ldaps().Return(repo).AnyTimes()

	configs := make([]*domain.Ldap, 2*platformpagination.MaxPageSize+1)
	for i := range configs {
		configs[i] = &domain.Ldap{ID: fmt.Sprintf("ldap-%03d", i), IsEnabled: true, SyncInterval: 5}
	}
	total := int64(len(configs))
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListQuery) ([]*domain.Ldap, int64, error) {
			assert.Equal(t, platformpagination.MaxPageSize, query.PageSize)
			offset := platformpagination.Offset(query.Page, query.PageSize)
			if offset >= len(configs) {
				return nil, total, nil
			}
			end := offset + query.PageSize
			if end > len(configs) {
				end = len(configs)
			}
			return configs[offset:end], total, nil
		}).Times(3)

	scheduler := &recordingSyncScheduler{}
	err := (Service{Scheduler: scheduler, UOW: uow}).ScheduleAllEnabledAutoSyncs(context.Background())
	require.NoError(t, err)
	require.Len(t, scheduler.ids, len(configs))
	assert.Equal(t, "ldap-200", scheduler.ids[len(scheduler.ids)-1])
}
