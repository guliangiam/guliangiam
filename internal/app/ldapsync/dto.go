package ldapsync

import (
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
)

// SyncConfigInput is the transport-agnostic input for creating a sync config.
// Transport adapters build this from their own request types so they never need
// to import the domain package.
type SyncConfigInput struct {
	OrganizationID      string
	LdapID              string
	SyncIntervalMinutes int
	BaseDN              string
	UserFilter          string
	GroupFilter         string
	Enabled             bool
}

// SyncConfigPatch is a partial update; nil fields are left untouched.
type SyncConfigPatch struct {
	SyncIntervalMinutes *int
	BaseDN              *string
	UserFilter          *string
	GroupFilter         *string
	Enabled             *bool
}

// SyncConfigView is the read model returned to callers.
type SyncConfigView struct {
	ID                  string     `json:"id"`
	OrganizationID      string     `json:"organizationId"`
	LdapID              string     `json:"ldapId"`
	SyncIntervalMinutes int        `json:"syncIntervalMinutes"`
	BaseDN              string     `json:"baseDn"`
	UserFilter          string     `json:"userFilter"`
	GroupFilter         string     `json:"groupFilter"`
	Enabled             bool       `json:"enabled"`
	LastSyncAt          *time.Time `json:"lastSyncAt,omitempty"`
	NextSyncAt          *time.Time `json:"nextSyncAt,omitempty"`
	SyncStatus          string     `json:"syncStatus"`
	SyncError           string     `json:"syncError,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	Revision            int64      `json:"revision"`
}

// SyncTaskView is the read model for a single sync execution record.
type SyncTaskView struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	LdapID         string     `json:"ldapId"`
	ConfigID       string     `json:"configId"`
	Status         string     `json:"status"`
	TriggerType    string     `json:"triggerType"`
	UsersSynced    int        `json:"usersSynced"`
	UsersCreated   int        `json:"usersCreated"`
	UsersUpdated   int        `json:"usersUpdated"`
	UsersFailed    int        `json:"usersFailed"`
	GroupsSynced   int        `json:"groupsSynced"`
	GroupsCreated  int        `json:"groupsCreated"`
	ErrorMessage   string     `json:"errorMessage,omitempty"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// SyncResultView is the read model for a completed sync operation.
type SyncResultView struct {
	UsersSynced   int    `json:"usersSynced"`
	UsersCreated  int    `json:"usersCreated"`
	UsersUpdated  int    `json:"usersUpdated"`
	UsersFailed   int    `json:"usersFailed"`
	GroupsSynced  int    `json:"groupsSynced"`
	GroupsCreated int    `json:"groupsCreated"`
	DurationMs    int64  `json:"durationMs"`
	Error         string `json:"error,omitempty"`
}

func toSyncConfigView(c *ldaps.LdapSyncConfig) SyncConfigView {
	return SyncConfigView{
		ID:                  c.ID,
		OrganizationID:      c.OrganizationID,
		LdapID:              c.LdapID,
		SyncIntervalMinutes: c.SyncIntervalMinutes,
		BaseDN:              c.BaseDN,
		UserFilter:          c.UserFilter,
		GroupFilter:         c.GroupFilter,
		Enabled:             c.Enabled,
		LastSyncAt:          c.LastSyncAt,
		NextSyncAt:          c.NextSyncAt,
		SyncStatus:          c.SyncStatus,
		SyncError:           c.SyncError,
		CreatedAt:           c.CreatedAt,
		UpdatedAt:           c.UpdatedAt,
		Revision:            c.Revision,
	}
}

func toSyncTaskView(t *ldaps.LdapSyncTask) SyncTaskView {
	return SyncTaskView{
		ID:             t.ID,
		OrganizationID: t.OrganizationID,
		LdapID:         t.LdapID,
		ConfigID:       t.ConfigID,
		Status:         t.Status,
		TriggerType:    t.TriggerType,
		UsersSynced:    t.UsersSynced,
		UsersCreated:   t.UsersCreated,
		UsersUpdated:   t.UsersUpdated,
		UsersFailed:    t.UsersFailed,
		GroupsSynced:   t.GroupsSynced,
		GroupsCreated:  t.GroupsCreated,
		ErrorMessage:   t.ErrorMessage,
		StartedAt:      t.StartedAt,
		CompletedAt:    t.CompletedAt,
		CreatedAt:      t.CreatedAt,
	}
}

func toSyncResultView(r *ldaps.LdapSyncResult) SyncResultView {
	return SyncResultView{
		UsersSynced:   r.UsersSynced,
		UsersCreated:  r.UsersCreated,
		UsersUpdated:  r.UsersUpdated,
		UsersFailed:   r.UsersFailed,
		GroupsSynced:  r.GroupsSynced,
		GroupsCreated: r.GroupsCreated,
		DurationMs:    r.Duration.Milliseconds(),
		Error:         r.Error,
	}
}
