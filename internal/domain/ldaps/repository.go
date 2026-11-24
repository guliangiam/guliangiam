package ldaps

import "context"

type ListQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type Repository interface {
	// GetByID is the system-scoped lookup used by authentication and background
	// sync workers, which run without a request tenant. Management routes must
	// use GetByIDForOrganization so a caller-supplied id cannot cross tenants.
	GetByID(ctx context.Context, id string) (*Ldap, error)
	// GetByIDForOrganization returns the LDAP configuration only when it belongs
	// to organizationID; a foreign or missing row is reported as not found.
	GetByIDForOrganization(ctx context.Context, id, organizationID string) (*Ldap, error)
	List(ctx context.Context, query ListQuery) ([]*Ldap, int64, error)
	Create(ctx context.Context, ldap *Ldap) error
	Update(ctx context.Context, ldap *Ldap) error
	// Delete removes the LDAP configuration only when it belongs to
	// organizationID; zero affected rows is reported as not found.
	Delete(ctx context.Context, id, organizationID string) error
}

type LDAPTestClient interface {
	TestConnection(host string, port int, bindDN, bindPassword string, useTLS bool) error
}

type SyncerConfig struct {
	ProviderType   string
	Host           string
	Port           int
	Username       string
	Password       string
	BaseDN         string
	Filter         string
	OrganizationID string
	SyncInterval   int
	// UseTLS is the operator's explicit SSL choice (ldaps.enable_ssl). It wins
	// over port inference so a directory on a non-standard port with SSL
	// enabled is still dialed over TLS instead of leaking the bind password.
	UseTLS bool
}

type LDAPPreviewUser struct {
	UUID        string `json:"uuid"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	ExistsLocal bool   `json:"existsLocal"`
}

type LDAPMappedUser struct {
	UUID           string `json:"uuid"`
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	GivenName      string `json:"givenName"`
	Surname        string `json:"surname"`
	Title          string `json:"title"`
	Department     string `json:"department"`
	EmployeeNumber string `json:"employeeNumber"`
	DN             string `json:"dn"`
	Disabled       bool   `json:"disabled"`
}

type LDAPMappedGroup struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type LDAPSyncer interface {
	PreviewUsersDomain(ctx context.Context, config *SyncerConfig) ([]*LDAPPreviewUser, error)
	GetMappedUsersDomain(ctx context.Context, config *SyncerConfig, uuids []string) ([]*LDAPMappedUser, error)
	GetMappedGroupsDomain(ctx context.Context, config *SyncerConfig) ([]*LDAPMappedGroup, error)
}

type SyncScheduler interface {
	AddDomainJob(syncerID string, config *SyncerConfig, syncFn func(ctx context.Context, config *SyncerConfig) error) error
	RemoveJob(syncerID string)
}
