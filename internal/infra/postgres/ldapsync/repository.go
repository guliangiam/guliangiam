package ldapsync

import (
	"context"
	"errors" // stdlib errors retained for errors.Is/errors.As only; apperrors used for error creation
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
)

var (
	ErrSyncConfigNotFound = apperrors.NewNotFound("ldap sync config not found")
	ErrSyncTaskNotFound   = apperrors.NewNotFound("ldap sync task not found")
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetSyncConfigByID(ctx context.Context, id string) (*ldaps.LdapSyncConfig, error) {
	query := `
		SELECT id, organization_id, ldap_id, sync_interval_minutes, base_dn,
		       user_filter, group_filter, enabled, last_sync_at, next_sync_at,
		       sync_status, sync_error, created_at, updated_at, revision
		FROM guliangiam.ldap_sync_configs
		WHERE id = $1
	`
	return r.scanSyncConfig(ctx, query, id)
}

func (r *Repository) GetSyncConfigByIDForOrganization(ctx context.Context, id, organizationID string) (*ldaps.LdapSyncConfig, error) {
	query := `
		SELECT id, organization_id, ldap_id, sync_interval_minutes, base_dn,
		       user_filter, group_filter, enabled, last_sync_at, next_sync_at,
		       sync_status, sync_error, created_at, updated_at, revision
		FROM guliangiam.ldap_sync_configs
		WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2)
	`
	return r.scanSyncConfig(ctx, query, id, organizationID)
}

func (r *Repository) scanSyncConfig(ctx context.Context, query string, args ...any) (*ldaps.LdapSyncConfig, error) {
	var config ldaps.LdapSyncConfig
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&config.ID, &config.OrganizationID, &config.LdapID,
		&config.SyncIntervalMinutes, &config.BaseDN,
		&config.UserFilter, &config.GroupFilter, &config.Enabled,
		&config.LastSyncAt, &config.NextSyncAt,
		&config.SyncStatus, &config.SyncError,
		&config.CreatedAt, &config.UpdatedAt, &config.Revision,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSyncConfigNotFound
		}
		return nil, apperrors.WrapInternalf(err, "get ldap sync config")
	}
	return &config, nil
}

func (r *Repository) ListSyncConfigs(ctx context.Context, organizationID string) ([]*ldaps.LdapSyncConfig, error) {
	query := `
		SELECT id, organization_id, ldap_id, sync_interval_minutes, base_dn,
		       user_filter, group_filter, enabled, last_sync_at, next_sync_at,
		       sync_status, sync_error, created_at, updated_at, revision
		FROM guliangiam.ldap_sync_configs
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, organizationID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list ldap sync configs")
	}
	defer rows.Close()

	var configs []*ldaps.LdapSyncConfig
	for rows.Next() {
		var config ldaps.LdapSyncConfig
		if err := rows.Scan(
			&config.ID, &config.OrganizationID, &config.LdapID,
			&config.SyncIntervalMinutes, &config.BaseDN,
			&config.UserFilter, &config.GroupFilter, &config.Enabled,
			&config.LastSyncAt, &config.NextSyncAt,
			&config.SyncStatus, &config.SyncError,
			&config.CreatedAt, &config.UpdatedAt, &config.Revision,
		); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan ldap sync config")
		}
		configs = append(configs, &config)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, apperrors.WrapInternalf(rowsErr, "iterate ldap sync configs")
	}
	return configs, nil
}

func (r *Repository) CreateSyncConfig(ctx context.Context, config *ldaps.LdapSyncConfig) error {
	query := `
		INSERT INTO guliangiam.ldap_sync_configs (
			id, organization_id, ldap_id, sync_interval_minutes, base_dn,
			user_filter, group_filter, enabled, sync_status, created_at, updated_at, revision
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'idle', $9, $10, 1)
	`
	now := time.Now()
	_, err := r.db.Exec(ctx, query,
		config.ID, config.OrganizationID, config.LdapID,
		config.SyncIntervalMinutes, config.BaseDN,
		config.UserFilter, config.GroupFilter, config.Enabled,
		now, now,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "create ldap sync config")
	}
	config.CreatedAt = now
	config.UpdatedAt = now
	return nil
}

func (r *Repository) UpdateSyncConfig(ctx context.Context, config *ldaps.LdapSyncConfig) error {
	query := `
		UPDATE guliangiam.ldap_sync_configs
		SET sync_interval_minutes = $2, base_dn = $3, user_filter = $4,
		    group_filter = $5, enabled = $6, updated_at = $7, revision = revision + 1
		WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($8)
		RETURNING revision
	`
	var newRevision int64
	err := r.db.QueryRow(ctx, query,
		config.ID, config.SyncIntervalMinutes, config.BaseDN,
		config.UserFilter, config.GroupFilter, config.Enabled,
		time.Now(), config.OrganizationID,
	).Scan(&newRevision)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSyncConfigNotFound
		}
		return apperrors.WrapInternalf(err, "update ldap sync config")
	}
	config.Revision = newRevision
	return nil
}

func (r *Repository) DeleteSyncConfig(ctx context.Context, id, organizationID string) error {
	result, err := r.db.Exec(ctx, `
		DELETE FROM guliangiam.ldap_sync_configs
		WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2)
	`, id, organizationID)
	if err != nil {
		return apperrors.WrapInternalf(err, "delete ldap sync config")
	}
	if result.RowsAffected() == 0 {
		return ErrSyncConfigNotFound
	}
	return nil
}

func (r *Repository) CreateSyncTask(ctx context.Context, task *ldaps.LdapSyncTask) error {
	query := `
		INSERT INTO guliangiam.ldap_sync_tasks (
			id, organization_id, ldap_id, config_id, status, trigger_type,
			users_synced, users_created, users_updated, users_failed,
			groups_synced, groups_created, error_message, started_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, 0, 0, 0, 0, 0, 0, '', $7, $8)
	`
	now := time.Now()
	_, err := r.db.Exec(ctx, query,
		task.ID, task.OrganizationID, task.LdapID, task.ConfigID,
		task.Status, task.TriggerType, now, now,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "create ldap sync task")
	}
	task.CreatedAt = now
	return nil
}

func (r *Repository) UpdateSyncTask(ctx context.Context, task *ldaps.LdapSyncTask) error {
	query := `
		UPDATE guliangiam.ldap_sync_tasks
		SET status = $2, users_synced = $3, users_created = $4, users_updated = $5,
		    users_failed = $6, groups_synced = $7, groups_created = $8,
		    error_message = $9, completed_at = $10
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query,
		task.ID, task.Status,
		task.UsersSynced, task.UsersCreated, task.UsersUpdated, task.UsersFailed,
		task.GroupsSynced, task.GroupsCreated, task.ErrorMessage, task.CompletedAt,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "update ldap sync task")
	}
	return nil
}

func (r *Repository) ListSyncTasks(ctx context.Context, ldapID string, limit int) ([]*ldaps.LdapSyncTask, error) {
	query := `
		SELECT id, organization_id, ldap_id, config_id, status, trigger_type,
		       users_synced, users_created, users_updated, users_failed,
		       groups_synced, groups_created, error_message, started_at, completed_at, created_at
		FROM guliangiam.ldap_sync_tasks
		WHERE ldap_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	return r.listSyncTasks(ctx, query, ldapID, limit)
}

func (r *Repository) ListSyncTasksForOrganization(ctx context.Context, ldapID, organizationID string, limit int) ([]*ldaps.LdapSyncTask, error) {
	query := `
		SELECT id, organization_id, ldap_id, config_id, status, trigger_type,
		       users_synced, users_created, users_updated, users_failed,
		       groups_synced, groups_created, error_message, started_at, completed_at, created_at
		FROM guliangiam.ldap_sync_tasks
		WHERE ldap_id = $1 AND organization_id = guliangiam.resolve_org_id($2)
		ORDER BY created_at DESC
		LIMIT $3
	`
	return r.listSyncTasks(ctx, query, ldapID, organizationID, limit)
}

func (r *Repository) listSyncTasks(ctx context.Context, query string, args ...any) ([]*ldaps.LdapSyncTask, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list ldap sync tasks")
	}
	defer rows.Close()

	var tasks []*ldaps.LdapSyncTask
	for rows.Next() {
		var task ldaps.LdapSyncTask
		if err := rows.Scan(
			&task.ID, &task.OrganizationID, &task.LdapID, &task.ConfigID,
			&task.Status, &task.TriggerType,
			&task.UsersSynced, &task.UsersCreated, &task.UsersUpdated, &task.UsersFailed,
			&task.GroupsSynced, &task.GroupsCreated, &task.ErrorMessage,
			&task.StartedAt, &task.CompletedAt, &task.CreatedAt,
		); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan ldap sync task")
		}
		tasks = append(tasks, &task)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, apperrors.WrapInternalf(rowsErr, "iterate ldap sync tasks")
	}
	return tasks, nil
}

func (r *Repository) GetLatestSyncTask(ctx context.Context, ldapID string) (*ldaps.LdapSyncTask, error) {
	tasks, err := r.ListSyncTasks(ctx, ldapID, 1)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, ErrSyncTaskNotFound
	}
	return tasks[0], nil
}

func (r *Repository) ListDueSyncConfigs(ctx context.Context) ([]*ldaps.LdapSyncConfig, error) {
	query := `
		SELECT id, organization_id, ldap_id, sync_interval_minutes, base_dn,
		       user_filter, group_filter, enabled, last_sync_at, next_sync_at,
		       sync_status, sync_error, created_at, updated_at, revision
		FROM guliangiam.ldap_sync_configs
		WHERE enabled = true AND sync_status = 'idle'
		      AND (next_sync_at IS NULL OR next_sync_at <= NOW())
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list due ldap sync configs")
	}
	defer rows.Close()

	var configs []*ldaps.LdapSyncConfig
	for rows.Next() {
		var config ldaps.LdapSyncConfig
		if err := rows.Scan(
			&config.ID, &config.OrganizationID, &config.LdapID,
			&config.SyncIntervalMinutes, &config.BaseDN,
			&config.UserFilter, &config.GroupFilter, &config.Enabled,
			&config.LastSyncAt, &config.NextSyncAt,
			&config.SyncStatus, &config.SyncError,
			&config.CreatedAt, &config.UpdatedAt, &config.Revision,
		); err != nil {
			return nil, apperrors.WrapInternalf(err, "scan due ldap sync config")
		}
		configs = append(configs, &config)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, apperrors.WrapInternalf(rowsErr, "iterate due ldap sync configs")
	}
	return configs, nil
}

// MarkSyncRunning marks a sync config as running.
func (r *Repository) MarkSyncRunning(ctx context.Context, id string) error {
	query := `
		UPDATE guliangiam.ldap_sync_configs
		SET sync_status = 'running', updated_at = NOW()
		WHERE id = $1 AND sync_status = 'idle'
	`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return apperrors.WrapInternalf(err, "mark sync running")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewInvalidArgumentf("sync config %s is not idle, cannot start", id)
	}
	return nil
}

// MarkSyncComplete marks a sync config as complete and schedules next run.
func (r *Repository) MarkSyncComplete(ctx context.Context, id, syncErr string) error {
	status := "idle"
	if syncErr != "" {
		status = "failed"
	}
	query := `
		UPDATE guliangiam.ldap_sync_configs
		SET sync_status = $2, sync_error = $3, last_sync_at = NOW(),
		    next_sync_at = NOW() + (sync_interval_minutes || ' minutes')::interval,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, status, syncErr)
	if err != nil {
		return apperrors.WrapInternalf(err, "mark sync complete")
	}
	return nil
}
