package ldap

import (
	"context"

	domainldaps "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Syncer implements domain.LDAPSyncer using the Go LDAP client library.
type Syncer struct {
	client *Client
}

// NewSyncer creates a new LDAP syncer.
func NewSyncer() *Syncer {
	return &Syncer{client: NewClient()}
}

// PreviewUsersDomain fetches users from the LDAP directory and returns them
// in a preview-friendly format (UUID, username, display name, email).
func (s *Syncer) PreviewUsersDomain(ctx context.Context, cfg *domainldaps.SyncerConfig) ([]*domainldaps.LDAPPreviewUser, error) {
	conn, err := s.client.Connect(cfg.Host, cfg.Port, cfg.Username, cfg.Password, syncerUseTLS(cfg))
	if err != nil {
		return nil, apperrors.NewUnavailablef("ldap connect failed: %v", err)
	}
	defer conn.Close()

	filter := cfg.Filter
	if filter == "" {
		filter = DefaultUserFilter(cfg.ProviderType)
	}

	ldapUsers, err := s.client.SearchUsers(conn, cfg.BaseDN, filter, nil)
	if err != nil {
		return nil, apperrors.NewUnavailablef("ldap search users failed: %v", err)
	}

	previews := make([]*domainldaps.LDAPPreviewUser, 0, len(ldapUsers))
	for _, u := range ldapUsers {
		previews = append(previews, &domainldaps.LDAPPreviewUser{
			UUID:        u.UID,
			Username:    u.UID,
			DisplayName: u.CommonName,
			Email:       u.Email,
		})
	}

	return previews, nil
}

// GetMappedUsersDomain fetches and maps LDAP users to the domain's mapped user format.
// If uuids is non-empty, only users matching those UUIDs are returned.
func (s *Syncer) GetMappedUsersDomain(ctx context.Context, cfg *domainldaps.SyncerConfig, uuids []string) ([]*domainldaps.LDAPMappedUser, error) {
	conn, err := s.client.Connect(cfg.Host, cfg.Port, cfg.Username, cfg.Password, syncerUseTLS(cfg))
	if err != nil {
		return nil, apperrors.NewUnavailablef("ldap connect failed: %v", err)
	}
	defer conn.Close()

	filter := cfg.Filter
	if filter == "" {
		filter = DefaultUserFilter(cfg.ProviderType)
	}

	ldapUsers, err := s.client.SearchUsers(conn, cfg.BaseDN, filter, nil)
	if err != nil {
		return nil, apperrors.NewUnavailablef("ldap search users failed: %v", err)
	}

	uuidSet := make(map[string]bool, len(uuids))
	for _, u := range uuids {
		uuidSet[u] = true
	}

	mapped := make([]*domainldaps.LDAPMappedUser, 0, len(ldapUsers))
	for _, u := range ldapUsers {
		if len(uuids) > 0 && !uuidSet[u.UID] {
			continue
		}
		mapped = append(mapped, &domainldaps.LDAPMappedUser{
			UUID:           u.UID,
			Username:       u.UID,
			DisplayName:    u.CommonName,
			Email:          u.Email,
			Phone:          u.Telephone,
			GivenName:      u.GivenName,
			Surname:        u.Surname,
			Title:          u.Title,
			Department:     u.Department,
			EmployeeNumber: u.EmployeeNumber,
			DN:             u.DN,
			Disabled:       u.Disabled,
		})
	}

	return mapped, nil
}

// GetMappedGroupsDomain fetches and maps LDAP groups to the domain's mapped group format.
func (s *Syncer) GetMappedGroupsDomain(ctx context.Context, cfg *domainldaps.SyncerConfig) ([]*domainldaps.LDAPMappedGroup, error) {
	conn, err := s.client.Connect(cfg.Host, cfg.Port, cfg.Username, cfg.Password, syncerUseTLS(cfg))
	if err != nil {
		return nil, apperrors.NewUnavailablef("ldap connect failed: %v", err)
	}
	defer conn.Close()

	groupFilter := DefaultGroupFilter(cfg.ProviderType)

	ldapGroups, err := s.client.SearchGroups(conn, cfg.BaseDN, groupFilter, nil)
	if err != nil {
		return nil, apperrors.NewUnavailablef("ldap search groups failed: %v", err)
	}

	mapped := make([]*domainldaps.LDAPMappedGroup, 0, len(ldapGroups))
	for _, g := range ldapGroups {
		mapped = append(mapped, &domainldaps.LDAPMappedGroup{
			Name:        g.CommonName,
			DisplayName: g.CommonName,
		})
	}

	return mapped, nil
}

// syncerUseTLS honors the operator's explicit SSL choice first and falls back
// to well-known-port inference for legacy configs without the flag.
func syncerUseTLS(cfg *domainldaps.SyncerConfig) bool {
	return cfg.UseTLS || ShouldUseTLS(cfg.Port)
}

var _ domainldaps.LDAPSyncer = (*Syncer)(nil)
