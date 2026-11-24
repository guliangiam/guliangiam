package ldapsync

import (
	"context"
	"log/slog"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// Scheduler runs periodic LDAP sync based on due configs.
type Scheduler struct {
	service  *Service
	interval time.Duration
	locks    LockProvider
	stopCh   chan struct{}
}

// NewScheduler creates a new LDAP sync scheduler. When locks is nil a no-op
// provider is used (single-replica deployments and tests).
func NewScheduler(service *Service, interval time.Duration, locks LockProvider) *Scheduler {
	if locks == nil {
		locks = noopLockProvider{}
	}
	return &Scheduler{
		service:  service,
		interval: interval,
		locks:    locks,
		stopCh:   make(chan struct{}),
	}
}

// Run starts the scheduler loop. Blocks until Stop is called.
func (s *Scheduler) Run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

// Stop signals the scheduler to stop.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// tick checks for due sync configs and executes them.
func (s *Scheduler) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// ldap_sync_configs/ldap_sync_tasks are FORCE RLS. A background context has
	// no tenant scope, so the pool's PrepareConn hook would fail closed (member
	// role + NULL org) and ListDueSyncConfigs would silently return zero rows.
	// The scheduler is a trusted cross-tenant component: run under System scope.
	ctx = tenantctx.System(ctx)

	configs, err := s.service.syncRepo.ListDueSyncConfigs(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "LDAP scheduler: failed to list due configs", "error", err)
		return
	}

	for _, config := range configs {
		if !config.Enabled {
			continue
		}

		// DB-061: without a cross-process lock every replica would sync the same
		// directory concurrently. The lock is per config and held for the whole
		// execution, not per transaction.
		release, acquired, lockErr := s.locks.TryLock(ctx, "ldapsync:config:"+config.ID)
		if lockErr != nil {
			slog.ErrorContext(ctx, "LDAP scheduler: cannot acquire lock",
				"config_id", config.ID,
				"error", lockErr,
			)
			continue
		}
		if !acquired {
			slog.InfoContext(ctx, "LDAP scheduler: config is being synced by another instance",
				"config_id", config.ID,
			)
			continue
		}
		s.runConfigLocked(ctx, config, release)
	}
}

// runConfigLocked executes one due config while holding its lock.
func (s *Scheduler) runConfigLocked(ctx context.Context, config *ldaps.LdapSyncConfig, release func(context.Context) error) {
	defer func() {
		if release == nil {
			return
		}
		// Use a fresh context: the caller's ctx may already be cancelled when the
		// tick deadline is reached, and the lock must still be released.
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := release(releaseCtx); err != nil {
			slog.ErrorContext(releaseCtx, "LDAP scheduler: cannot release lock", "config_id", config.ID, "error", err)
		}
	}()

	slog.InfoContext(ctx, "LDAP scheduler: executing scheduled sync",
		"config_id", config.ID,
		"ldap_id", config.LdapID,
	)

	// Create sync task
	taskID, err := newUUID()
	if err != nil {
		slog.ErrorContext(ctx, "LDAP scheduler: generate task ID", "error", err)
		return
	}

	now := time.Now()

	// Mark config as running
	if err := s.service.syncRepo.MarkSyncRunning(ctx, config.ID); err != nil {
		slog.ErrorContext(ctx, "LDAP scheduler: cannot mark sync running",
			"config_id", config.ID,
			"error", err,
		)
		return
	}

	// Execute sync
	result, err := s.service.executeSync(ctx, config)
	if err != nil {
		slog.ErrorContext(ctx, "LDAP scheduler: sync failed",
			"config_id", config.ID,
			"error", err,
		)
		if markErr := s.service.syncRepo.MarkSyncComplete(ctx, config.ID, err.Error()); markErr != nil {
			slog.WarnContext(ctx, "LDAP scheduler: mark failed sync complete",
				"config_id", config.ID, "error", markErr)
		}
	} else {
		if markErr := s.service.syncRepo.MarkSyncComplete(ctx, config.ID, ""); markErr != nil {
			slog.WarnContext(ctx, "LDAP scheduler: mark sync complete",
				"config_id", config.ID, "error", markErr)
		}

		// Log task result
		if createErr := s.service.syncRepo.CreateSyncTask(ctx, ldapSyncTaskFromResult(taskID, config, result, "scheduled", &now)); createErr != nil {
			slog.WarnContext(ctx, "LDAP scheduler: persist sync task failed",
				"config_id", config.ID, "task_id", taskID, "error", createErr)
		}
	}
}

func newUUID() (string, error) {
	return uuid.V7Generator{}.NewString()
}

func ldapSyncTaskFromResult(id string, config *ldaps.LdapSyncConfig, result *ldaps.LdapSyncResult, triggerType string, startedAt *time.Time) *ldaps.LdapSyncTask {
	completedAt := time.Now()
	status := "succeeded"
	if result.UsersFailed > 0 {
		status = "partial"
	}
	return &ldaps.LdapSyncTask{
		ID:             id,
		OrganizationID: config.OrganizationID,
		LdapID:         config.LdapID,
		ConfigID:       config.ID,
		Status:         status,
		TriggerType:    triggerType,
		UsersSynced:    result.UsersSynced,
		UsersCreated:   result.UsersCreated,
		UsersUpdated:   result.UsersUpdated,
		UsersFailed:    result.UsersFailed,
		GroupsSynced:   result.GroupsSynced,
		GroupsCreated:  result.GroupsCreated,
		StartedAt:      startedAt,
		CompletedAt:    &completedAt,
	}
}
