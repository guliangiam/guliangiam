package syncer

import (
	"context"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ActiveDirectorySyncer syncs users and groups from Active Directory.
// Since AD is LDAP-compatible, this syncer delegates to LDAPSyncer.
type ActiveDirectorySyncer struct {
	ldapSyncer *LDAPSyncer
}

// NewActiveDirectorySyncer creates a new ActiveDirectory syncer.
func NewActiveDirectorySyncer() *ActiveDirectorySyncer {
	return &ActiveDirectorySyncer{
		ldapSyncer: NewLDAPSyncer(),
	}
}

// getLDAPSyncer returns the underlying LDAP syncer.
func (s *ActiveDirectorySyncer) getLDAPSyncer() *LDAPSyncer {
	return s.ldapSyncer
}

// SyncUsers synchronizes users from Active Directory via LDAP.
// AD uses LDAP as its native protocol, so we delegate to LDAPSyncer.
// Typical AD LDAP settings:
//   - Host: AD domain controller (e.g., dc.example.com)
//   - Port: 389 (LDAP) or 636 (LDAPS)
//   - BaseDN: e.g., DC=example,DC=com
//   - Username: bind DN (e.g., CN=admin,DC=example,DC=com)
//   - Filter: e.g., (objectClass=user) or (objectClass=person)
func (s *ActiveDirectorySyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	// Apply AD-specific defaults to the config if not set
	adConfig := s.applyADDefaults(config)

	result, err := s.getLDAPSyncer().SyncUsers(ctx, adConfig)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "activedirectory syncer sync users")
	}
	return result, nil
}

// SyncGroups synchronizes groups from Active Directory via LDAP.
// AD groups are represented as LDAP entries with objectClass=group.
// Typical filter: (objectClass=group)
func (s *ActiveDirectorySyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	adConfig := s.applyADDefaults(config)

	result, err := s.getLDAPSyncer().SyncGroups(ctx, adConfig)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "activedirectory syncer sync groups")
	}
	return result, nil
}

// TestConnection tests the connection to Active Directory via LDAP.
func (s *ActiveDirectorySyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	adConfig := s.applyADDefaults(config)

	err := s.getLDAPSyncer().TestConnection(ctx, adConfig)
	if err != nil {
		return apperrors.WrapInternalf(err, "activedirectory test connection failed")
	}
	return nil
}

// applyADDefaults applies Active Directory-specific defaults to the config.
// This ensures that the LDAP syncer has appropriate defaults for AD,
// such as setting the default port to 389 and default user/group filters.
func (s *ActiveDirectorySyncer) applyADDefaults(config *SyncerConfig) *SyncerConfig {
	// Create a copy to avoid mutating the original config
	adConfig := *config

	// AD default port is 389 (LDAP) or 636 (LDAPS)
	if adConfig.Port == 0 {
		adConfig.Port = 389
	}

	// Apply AD-specific default user filter if not set
	// AD users have objectClass=user (or objectClass=person for broader match)
	if adConfig.Filter == "" {
		adConfig.Filter = "(objectClass=user)"
	}

	return &adConfig
}

// Ensure ActiveDirectorySyncer implements Engine.
var _ Engine = (*ActiveDirectorySyncer)(nil)
