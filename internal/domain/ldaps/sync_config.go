package ldaps

import (
	"context"
	"time"
)

// LdapSyncConfig represents the configuration for automatic LDAP synchronization.
type LdapSyncConfig struct {
	ID                  string
	OrganizationID      string
	LdapID              string
	SyncIntervalMinutes int
	BaseDN              string
	UserFilter          string
	GroupFilter         string
	Enabled             bool
	LastSyncAt          *time.Time
	NextSyncAt          *time.Time
	SyncStatus          string // idle, running, failed
	SyncError           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Revision            int64
}

// LdapSyncTask represents a single sync execution record.
type LdapSyncTask struct {
	ID             string
	OrganizationID string
	LdapID         string
	ConfigID       string
	Status         string // pending, running, succeeded, failed
	TriggerType    string // manual, scheduled
	UsersSynced    int
	UsersCreated   int
	UsersUpdated   int
	UsersFailed    int
	GroupsSynced   int
	GroupsCreated  int
	ErrorMessage   string
	StartedAt      *time.Time
	CompletedAt    *time.Time
	CreatedAt      time.Time
}

// LdapSyncResult represents the result of a sync operation.
type LdapSyncResult struct {
	UsersSynced   int
	UsersCreated  int
	UsersUpdated  int
	UsersFailed   int
	GroupsSynced  int
	GroupsCreated int
	Duration      time.Duration
	Error         string
}

// Repository defines the interface for LDAP sync operations.
type SyncRepository interface {
	// Config operations
	//
	// GetSyncConfigByID is the system-scoped lookup used by the background
	// scheduler and one-off operational tooling. Management routes, including
	// the manual sync trigger (POST /ldap-sync-configs/{id}/sync), must use
	// GetSyncConfigByIDForOrganization so a caller-supplied id cannot cross
	// tenants.
	GetSyncConfigByID(ctx context.Context, id string) (*LdapSyncConfig, error)
	// GetSyncConfigByIDForOrganization returns the sync config only when it
	// belongs to organizationID; a foreign or missing row is reported as not
	// found.
	GetSyncConfigByIDForOrganization(ctx context.Context, id, organizationID string) (*LdapSyncConfig, error)
	ListSyncConfigs(ctx context.Context, organizationID string) ([]*LdapSyncConfig, error)
	CreateSyncConfig(ctx context.Context, config *LdapSyncConfig) error
	// UpdateSyncConfig scopes the written row by the entity's own
	// OrganizationID, not by a caller-supplied value.
	UpdateSyncConfig(ctx context.Context, config *LdapSyncConfig) error
	// DeleteSyncConfig removes the config only when it belongs to
	// organizationID; zero affected rows is reported as not found.
	DeleteSyncConfig(ctx context.Context, id, organizationID string) error

	// Task operations
	CreateSyncTask(ctx context.Context, task *LdapSyncTask) error
	UpdateSyncTask(ctx context.Context, task *LdapSyncTask) error
	// ListSyncTasks keeps the system-scoped task lookup used by background
	// workers (GetLatestSyncTask).
	ListSyncTasks(ctx context.Context, ldapID string, limit int) ([]*LdapSyncTask, error)
	// ListSyncTasksForOrganization returns tasks for ldapID only when they
	// belong to organizationID; management log reads use it.
	ListSyncTasksForOrganization(ctx context.Context, ldapID, organizationID string, limit int) ([]*LdapSyncTask, error)
	GetLatestSyncTask(ctx context.Context, ldapID string) (*LdapSyncTask, error)

	// Scheduling
	ListDueSyncConfigs(ctx context.Context) ([]*LdapSyncConfig, error)
	MarkSyncRunning(ctx context.Context, id string) error
	MarkSyncComplete(ctx context.Context, id string, syncErr string) error
}
