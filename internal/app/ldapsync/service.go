package ldapsync

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// Service provides LDAP sync business logic.
type Service struct {
	syncRepo ldaps.SyncRepository
	ldapRepo ldaps.Repository
	userRepo users.Repository
	locks    LockProvider

	mu        sync.Mutex
	scheduler *Scheduler
}

func NewService(syncRepo ldaps.SyncRepository, ldapRepo ldaps.Repository, userRepo users.Repository) *Service {
	return &Service{
		syncRepo: syncRepo,
		ldapRepo: ldapRepo,
		userRepo: userRepo,
		locks:    noopLockProvider{},
	}
}

// WithLockProvider injects the cross-process lock used by the scheduler
// (DB-061). Returns the receiver so it can be chained at wiring time.
func (s *Service) WithLockProvider(locks LockProvider) *Service {
	if locks != nil {
		s.locks = locks
	}
	return s
}

// StartScheduler starts the background sync scheduler.
func (s *Service) StartScheduler() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scheduler != nil {
		return
	}
	s.scheduler = NewScheduler(s, 30*time.Second, s.locks)
	recover.SafeGo("ldapsync-scheduler", s.scheduler.Run)
	slog.Info("LDAP sync scheduler started")
}

// StopScheduler stops the background sync scheduler.
func (s *Service) StopScheduler() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scheduler != nil {
		s.scheduler.Stop()
		s.scheduler = nil
		slog.Info("LDAP sync scheduler stopped")
	}
}

// CreateSyncConfig creates a new LDAP sync configuration and returns its ID.
func (s *Service) CreateSyncConfig(ctx context.Context, input SyncConfigInput) (string, error) {
	// BE-B-17: a zero/negative interval makes the config permanently due (the
	// scheduler would sync it on every tick); reject it at the boundary.
	if input.SyncIntervalMinutes < 1 {
		return "", apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "syncIntervalMinutes must be at least 1", map[string]any{
			"field": "syncIntervalMinutes", "value": input.SyncIntervalMinutes, "min": 1,
		})
	}
	if strings.TrimSpace(input.LdapID) == "" {
		return "", apperrors.NewInvalidArgument("ldapId is required")
	}
	// BE-B-17: the referenced LDAP connection must belong to the trusted
	// tenant; a caller-supplied foreign ldapId must not attach another
	// organization's directory credentials to this org's sync config.
	if s.ldapRepo != nil {
		if _, err := s.ldapRepo.GetByIDForOrganization(ctx, input.LdapID, input.OrganizationID); err != nil {
			return "", err
		}
	}
	id, err := uuid.V7Generator{}.NewString()
	if err != nil {
		return "", apperrors.WrapInternal(err, "generate sync config id")
	}
	config := &ldaps.LdapSyncConfig{
		ID:                  id,
		OrganizationID:      input.OrganizationID,
		LdapID:              input.LdapID,
		SyncIntervalMinutes: input.SyncIntervalMinutes,
		BaseDN:              input.BaseDN,
		UserFilter:          input.UserFilter,
		GroupFilter:         input.GroupFilter,
		Enabled:             input.Enabled,
	}
	if err := s.syncRepo.CreateSyncConfig(ctx, config); err != nil {
		return "", err
	}
	return id, nil
}

// GetSyncConfig retrieves a sync config by ID within the trusted tenant.
func (s *Service) GetSyncConfig(ctx context.Context, organizationID, id string) (*SyncConfigView, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	config, err := s.syncRepo.GetSyncConfigByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	view := toSyncConfigView(config)
	return &view, nil
}

// ListSyncConfigs lists all sync configs for an organization.
func (s *Service) ListSyncConfigs(ctx context.Context, organizationID string) ([]SyncConfigView, error) {
	configs, err := s.syncRepo.ListSyncConfigs(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	views := make([]SyncConfigView, 0, len(configs))
	for _, c := range configs {
		views = append(views, toSyncConfigView(c))
	}
	return views, nil
}

// PatchSyncConfig applies a partial update to an existing sync configuration.
// Ownership is decided before mutation: a foreign or missing id returns
// NotFound and leaves no write behind. UpdateSyncConfig re-checks the entity's
// organization in the same UPDATE statement.
func (s *Service) PatchSyncConfig(ctx context.Context, organizationID, id string, patch SyncConfigPatch) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	config, err := s.syncRepo.GetSyncConfigByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return err
	}
	if patch.SyncIntervalMinutes != nil {
		// BE-B-17: same floor as creation — a patched zero/negative interval
		// would make the config permanently due.
		if *patch.SyncIntervalMinutes < 1 {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "syncIntervalMinutes must be at least 1", map[string]any{
				"field": "syncIntervalMinutes", "value": *patch.SyncIntervalMinutes, "min": 1,
			})
		}
		config.SyncIntervalMinutes = *patch.SyncIntervalMinutes
	}
	if patch.BaseDN != nil {
		config.BaseDN = *patch.BaseDN
	}
	if patch.UserFilter != nil {
		config.UserFilter = *patch.UserFilter
	}
	if patch.GroupFilter != nil {
		config.GroupFilter = *patch.GroupFilter
	}
	if patch.Enabled != nil {
		config.Enabled = *patch.Enabled
	}
	return s.syncRepo.UpdateSyncConfig(ctx, config)
}

// DeleteSyncConfig deletes a sync configuration within the trusted tenant.
// The scoped read rejects a foreign id before the delete; the delete
// predicate then re-checks the entity's real organization. This domain has no
// audit/outbox path, so no audit record is emitted.
func (s *Service) DeleteSyncConfig(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	config, err := s.syncRepo.GetSyncConfigByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return err
	}
	return s.syncRepo.DeleteSyncConfig(ctx, id, config.OrganizationID)
}

// TriggerSync manually triggers a sync for a config inside the trusted tenant.
// Ownership is resolved before the sync task is created or any directory
// connection is opened. This domain has no UnitOfWork/audit path; the scoped
// config read plus organization-predicated writes are the tenant boundary.
func (s *Service) TriggerSync(ctx context.Context, organizationID, configID string) (*SyncResultView, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	config, err := s.syncRepo.GetSyncConfigByIDForOrganization(ctx, configID, organizationID)
	if err != nil {
		return nil, err
	}

	// Create sync task
	taskID, err := uuid.V7Generator{}.NewString()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate sync task id")
	}
	now := time.Now()
	task := &ldaps.LdapSyncTask{
		ID:             taskID,
		OrganizationID: config.OrganizationID,
		LdapID:         config.LdapID,
		ConfigID:       configID,
		Status:         "running",
		TriggerType:    "manual",
		StartedAt:      &now,
	}
	if err := s.syncRepo.CreateSyncTask(ctx, task); err != nil {
		return nil, apperrors.WrapInternal(err, "create sync task")
	}

	// BE-B-18: the directory sync must not run inside the HTTP request. A slow
	// or unreachable LDAP server would hold the request (and its connection)
	// until client/proxy timeouts, and a client disconnect would cancel the
	// sync mid-write. Execute on a detached, bounded context under the
	// config's trusted tenant; the caller observes progress via the sync logs.
	asyncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	asyncCtx = tenantctx.WithTenantScope(asyncCtx, config.OrganizationID, "", tenantctx.TenantFilterStrict)
	recover.SafeGo("ldapsync-manual-sync", func() {
		defer cancel()
		result, execErr := s.executeSync(asyncCtx, config)
		if execErr != nil {
			task.Status = "failed"
			task.ErrorMessage = execErr.Error()
			if updateErr := s.syncRepo.UpdateSyncTask(asyncCtx, task); updateErr != nil {
				slog.WarnContext(asyncCtx, "ldap sync: persist failed task failed",
					"task_id", task.ID, "error", updateErr)
			}
			return
		}

		task.Status = "succeeded"
		task.UsersSynced = result.UsersSynced
		task.UsersCreated = result.UsersCreated
		task.UsersUpdated = result.UsersUpdated
		task.GroupsSynced = result.GroupsSynced
		task.GroupsCreated = result.GroupsCreated
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		if updateErr := s.syncRepo.UpdateSyncTask(asyncCtx, task); updateErr != nil {
			slog.WarnContext(asyncCtx, "ldap sync: persist completed task failed",
				"task_id", task.ID, "error", updateErr)
		}
	})

	// The task is queued; counters are populated asynchronously and read back
	// through GetSyncLogs. The response shape is unchanged.
	return &SyncResultView{}, nil
}

// GetSyncLogs retrieves sync task logs for the config identified by configID.
// The config is resolved in the trusted tenant first; tasks are then filtered
// by both ldap id and organization.
func (s *Service) GetSyncLogs(ctx context.Context, organizationID, configID string, limit int) ([]SyncTaskView, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	config, err := s.syncRepo.GetSyncConfigByIDForOrganization(ctx, configID, organizationID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.syncRepo.ListSyncTasksForOrganization(ctx, config.LdapID, config.OrganizationID, limit)
	if err != nil {
		return nil, err
	}
	views := make([]SyncTaskView, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, toSyncTaskView(t))
	}
	return views, nil
}

// executeSync performs the actual LDAP synchronization.
func (s *Service) executeSync(ctx context.Context, config *ldaps.LdapSyncConfig) (*ldaps.LdapSyncResult, error) {
	start := time.Now()
	result := &ldaps.LdapSyncResult{}

	// Get LDAP connection details
	ldapConfig, err := s.ldapRepo.GetByID(ctx, config.LdapID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get ldap config")
	}

	// Connect to LDAP server
	conn, err := connectLDAP(ldapConfig)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "connect LDAP")
	}
	defer conn.Close()

	// Determine base DN and filter
	baseDN := config.BaseDN
	if baseDN == "" {
		baseDN = ldapConfig.BaseDN
	}
	filter := config.UserFilter
	if filter == "" {
		filter = ldapConfig.Filter
	}

	// Search for users
	ldapEntries, err := searchUsers(conn, baseDN, filter, defaultAttributeMapping(ldapConfig.Type))
	if err != nil {
		return nil, apperrors.WrapInternal(err, "search LDAP users")
	}

	slog.InfoContext(ctx, "LDAP sync: found users",
		"ldap_id", ldapConfig.ID,
		"count", len(ldapEntries),
	)

	// Sync users to local database
	s.syncUsers(ctx, ldapConfig, ldapEntries, result)

	result.Duration = time.Since(start)

	slog.InfoContext(ctx, "LDAP sync completed",
		"ldap_id", ldapConfig.ID,
		"synced", result.UsersSynced,
		"created", result.UsersCreated,
		"updated", result.UsersUpdated,
		"failed", result.UsersFailed,
		"duration", result.Duration,
	)

	return result, nil
}
