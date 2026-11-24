package syncer

import (
	"context"
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domainldaps "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/robfig/cron/v3"
)

// Scheduler manages periodic sync jobs.
type Scheduler struct {
	cron    *cron.Cron
	factory *Factory
	jobs    map[string]cron.EntryID // syncerID -> cron entry ID
}

// NewScheduler creates a new sync scheduler.
func NewScheduler(factory *Factory) *Scheduler {
	return &Scheduler{
		cron:    cron.New(cron.WithSeconds()),
		factory: factory,
		jobs:    make(map[string]cron.EntryID),
	}
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// AddJob adds a periodic sync job.
func (s *Scheduler) AddJob(syncerID string, config *SyncerConfig, syncFn func(ctx context.Context, config *SyncerConfig) error) error {
	if _, exists := s.jobs[syncerID]; exists {
		s.RemoveJob(syncerID)
	}

	interval := config.SyncInterval
	if interval <= 0 {
		interval = 3600 // default: 1 hour
	}

	spec := fmt.Sprintf("@every %ds", interval)
	entryID, err := s.cron.AddFunc(spec, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = syncFn(ctx, config)
	})
	if err != nil {
		return apperrors.WrapInternalf(err, "add sync job")
	}

	s.jobs[syncerID] = entryID
	return nil
}

func (s *Scheduler) RemoveJob(syncerID string) {
	if entryID, exists := s.jobs[syncerID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, syncerID)
	}
}

var _ domainldaps.SyncScheduler = (*Scheduler)(nil)

func (s *Scheduler) AddDomainJob(syncerID string, config *domainldaps.SyncerConfig, syncFn func(ctx context.Context, config *domainldaps.SyncerConfig) error) error {
	infraConfig := &SyncerConfig{
		ProviderType:   config.ProviderType,
		Host:           config.Host,
		Port:           config.Port,
		Username:       config.Username,
		Password:       config.Password,
		BaseDN:         config.BaseDN,
		Filter:         config.Filter,
		OrganizationID: config.OrganizationID,
		SyncInterval:   config.SyncInterval,
	}
	return s.AddJob(syncerID, infraConfig, func(ctx context.Context, cfg *SyncerConfig) error {
		return syncFn(ctx, config)
	})
}
