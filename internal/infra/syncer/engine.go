package syncer

import (
	"context"
	"time"

	domainsyncers "github.com/guliangiam/guliangiam/internal/domain/syncers"
)

type UserWriteBackFunc = domainsyncers.UserWriteBackFunc
type GroupWriteBackFunc = domainsyncers.GroupWriteBackFunc

type Engine interface {
	SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error)
	SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error)
	TestConnection(ctx context.Context, config *SyncerConfig) error
}

type SyncerConfig struct {
	ProviderType   string             `json:"providerType"`
	Host           string             `json:"host"`
	Port           int                `json:"port"`
	BaseDN         string             `json:"baseDN"`
	Username       string             `json:"username"`
	Password       string             `json:"password"`
	Filter         string             `json:"filter"`
	AttributeMap   map[string]string  `json:"attributeMap"`
	OrganizationID string             `json:"organizationId"`
	EnableSync     bool               `json:"enableSync"`
	SyncInterval   int                `json:"syncInterval"`
	IsReadOnly     bool               `json:"isReadOnly"`
	LastSyncAt     *time.Time         `json:"lastSyncAt"`
	ClientID       string             `json:"clientId"`
	ClientSecret   string             `json:"clientSecret"`
	TenantID       string             `json:"tenantId"`
	TokenURL       string             `json:"tokenUrl"`
	APIURL         string             `json:"apiUrl"`
	OnUserSynced   UserWriteBackFunc  `json:"-"`
	OnGroupSynced  GroupWriteBackFunc `json:"-"`
}

type SyncResult struct {
	TotalUsers    int      `json:"totalUsers"`
	AddedUsers    int      `json:"addedUsers"`
	UpdatedUsers  int      `json:"updatedUsers"`
	FailedUsers   int      `json:"failedUsers"`
	TotalGroups   int      `json:"totalGroups"`
	AddedGroups   int      `json:"addedGroups"`
	UpdatedGroups int      `json:"updatedGroups"`
	FailedGroups  int      `json:"failedGroups"`
	Errors        []string `json:"errors,omitempty"`
}

type FieldMapping struct {
	SourceField string `json:"sourceField"`
	TargetField string `json:"targetField"`
	Required    bool   `json:"required"`
}
