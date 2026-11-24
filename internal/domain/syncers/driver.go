package syncers

import (
	"context"
	"time"
)

// UserWriteBackFunc is a callback that persists a mapped user to the local database.
type UserWriteBackFunc func(ctx context.Context, providerID string, externalID string, mappedUser map[string]interface{}) (created bool, err error)

// GroupWriteBackFunc is a callback that persists a mapped group to the local database.
type GroupWriteBackFunc func(ctx context.Context, organizationID string, externalID string, mappedGroup map[string]interface{}) (created bool, err error)

// ExternalUser represents a user from an external identity source.
// It maps to GuliangIAM's User aggregate through field mapping.
type ExternalUser struct {
	ID            string
	Name          string
	DisplayName   string
	Email         string
	Phone         string
	Password      string
	PasswordSalt  string
	PasswordType  string
	Avatar        string
	FirstName     string
	LastName      string
	Affiliation   string
	Title         string
	Location      string
	Address       string
	Region        string
	Language      string
	Gender        string
	Birthday      string
	Education     string
	Bio           string
	Homepage      string
	Tag           string
	IsAdmin       bool
	IsForbidden   bool
	IsDeleted     bool
	IsVerified    bool
	Type          string
	Groups        []string
	Properties    map[string]string
	RawAttributes map[string]interface{} // unmapped attributes from source
}

// ExternalGroup represents a group from an external identity source.
type ExternalGroup struct {
	ID          string
	Name        string
	DisplayName string
	Description string
	Type        string
	Manager     string
	Email       string
	ParentID    string
}

// SyncResult captures the outcome of a sync operation.
type SyncResult struct {
	UsersAdded    int
	UsersUpdated  int
	UsersDeleted  int
	UsersSkipped  int
	GroupsAdded   int
	GroupsUpdated int
	GroupsDeleted int
	Errors        []SyncError
}

// SyncError records a single error during sync.
type SyncError struct {
	Entity     string // "user" or "group"
	Identifier string
	Message    string
}

// SyncerDriver defines the interface that all syncer implementations must satisfy.
// Each external source type (AD, Azure AD, Keycloak, etc.) implements this interface.
type SyncerDriver interface {
	FetchUsers(ctx context.Context) ([]*ExternalUser, error)
	FetchGroups(ctx context.Context) ([]*ExternalGroup, error)
	FetchUserGroups(ctx context.Context, userID string) ([]string, error)
	AddUser(ctx context.Context, user *ExternalUser) error
	UpdateUser(ctx context.Context, user *ExternalUser) error
	TestConnection(ctx context.Context) error
	Close() error
}

type SyncEngineConfig struct {
	ProviderType   string
	Host           string
	Port           int
	BaseDN         string
	Username       string
	Password       string
	Filter         string
	AttributeMap   map[string]string
	OrganizationID string
	EnableSync     bool
	SyncInterval   int
	IsReadOnly     bool
	LastSyncAt     *time.Time
	ClientID       string
	ClientSecret   string
	TenantID       string
	TokenURL       string
	APIURL         string
	OnUserSynced   UserWriteBackFunc
	OnGroupSynced  GroupWriteBackFunc
}

type SyncEngineResult struct {
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

type SyncEngine interface {
	SyncUsers(ctx context.Context, config *SyncEngineConfig) (*SyncEngineResult, error)
	SyncGroups(ctx context.Context, config *SyncEngineConfig) (*SyncEngineResult, error)
	TestConnection(ctx context.Context, config *SyncEngineConfig) error
}

type SyncEngineFactory interface {
	GetEngine(providerType string) (SyncEngine, error)
}
