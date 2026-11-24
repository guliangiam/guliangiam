package ldapsync

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/redact"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	// ldapDialTimeout bounds the TCP/TLS handshake to the directory.
	ldapDialTimeout = 10 * time.Second
	// ldapSearchSizeLimit caps one directory search so a broad filter cannot
	// pull an unbounded result set into memory.
	ldapSearchSizeLimit = 10000
)

// AttributeMapping defines how LDAP attributes map to local user fields.
type AttributeMapping struct {
	Username    string // LDAP attribute for username (default: uid / sAMAccountName)
	DisplayName string // LDAP attribute for display name (default: cn / displayName)
	Email       string // LDAP attribute for email (default: mail)
	Phone       string // LDAP attribute for phone (default: telephoneNumber)
	FirstName   string // LDAP attribute for first name (default: givenName)
	LastName    string // LDAP attribute for last name (default: sn)
}

// defaultAttributeMapping returns the default attribute mapping based on LDAP server type.
func defaultAttributeMapping(ldapType string) AttributeMapping {
	if strings.EqualFold(ldapType, "ActiveDirectory") {
		return AttributeMapping{
			Username:    "sAMAccountName",
			DisplayName: "displayName",
			Email:       "mail",
			Phone:       "telephoneNumber",
			FirstName:   "givenName",
			LastName:    "sn",
		}
	}
	// OpenLDAP / FreeIPA default
	return AttributeMapping{
		Username:    "uid",
		DisplayName: "cn",
		Email:       "mail",
		Phone:       "telephoneNumber",
		FirstName:   "givenName",
		LastName:    "sn",
	}
}

// ldapEntry holds extracted data from a single LDAP user entry.
type ldapEntry struct {
	Username    string
	DisplayName string
	Email       string
	Phone       string
	FirstName   string
	LastName    string
	DN          string
}

// ldapURL builds the connection URL used by ldap.DialURL, mirroring the
// infrastructure layer's host:port convention with the scheme selected by TLS.
func ldapURL(host string, port int, useTLS bool) string {
	if useTLS {
		return fmt.Sprintf("ldaps://%s:%d", host, port)
	}
	return fmt.Sprintf("ldap://%s:%d", host, port)
}

// connectLDAP establishes a connection to the LDAP server and binds.
func connectLDAP(ldapConfig *ldaps.Ldap) (*ldap.Conn, error) {
	url := ldapURL(ldapConfig.Host, ldapConfig.Port, ldapConfig.EnableSsl)

	var conn *ldap.Conn
	var err error

	// Bound the dial itself: an unreachable host must surface as an error in
	// seconds, not hang until the OS TCP timeout.
	dialer := ldap.DialWithDialer(&net.Dialer{Timeout: ldapDialTimeout})
	if ldapConfig.EnableSsl {
		if ldapConfig.AllowSelfSignedCert {
			slog.Warn("ldap sync skips TLS verification for a self-signed directory",
				"host", ldapConfig.Host, "port", ldapConfig.Port)
		}
		tlsConfig := &tls.Config{
			ServerName:         ldapConfig.ServerName,
			InsecureSkipVerify: ldapConfig.AllowSelfSignedCert, // #nosec G402 -- operator opt-in for self-signed directory servers
			MinVersion:         tls.VersionTLS12,
		}
		conn, err = ldap.DialURL(url, dialer, ldap.DialWithTLSConfig(tlsConfig))
	} else {
		conn, err = ldap.DialURL(url, dialer)
	}

	if err != nil {
		return nil, apperrors.WrapInternal(err, "connect to LDAP server "+url)
	}

	// Bind with admin credentials
	if err := conn.Bind(ldapConfig.BindDN, ldapConfig.BindPassword); err != nil {
		conn.Close()
		return nil, apperrors.WrapInternal(err, "bind to LDAP server")
	}

	return conn, nil
}

// searchUsers queries the LDAP server for user entries.
func searchUsers(conn *ldap.Conn, baseDN, filter string, mapping AttributeMapping) ([]*ldapEntry, error) {
	if filter == "" {
		filter = "(objectClass=person)"
	}

	// Request the attributes we need
	attributes := []string{
		mapping.Username, mapping.DisplayName, mapping.Email,
		mapping.Phone, mapping.FirstName, mapping.LastName,
		"dn",
	}
	// Remove duplicates and empty attributes
	seen := make(map[string]bool)
	cleanAttrs := make([]string, 0, len(attributes))
	for _, attr := range attributes {
		if attr == "" || seen[strings.ToLower(attr)] {
			continue
		}
		seen[strings.ToLower(attr)] = true
		cleanAttrs = append(cleanAttrs, attr)
	}

	searchRequest := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		ldapSearchSizeLimit, 0, false,
		filter,
		cleanAttrs,
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "search LDAP users")
	}

	entries := make([]*ldapEntry, 0, len(sr.Entries))
	for _, e := range sr.Entries {
		entry := &ldapEntry{
			Username:    e.GetAttributeValue(mapping.Username),
			DisplayName: e.GetAttributeValue(mapping.DisplayName),
			Email:       e.GetAttributeValue(mapping.Email),
			Phone:       e.GetAttributeValue(mapping.Phone),
			FirstName:   e.GetAttributeValue(mapping.FirstName),
			LastName:    e.GetAttributeValue(mapping.LastName),
			DN:          e.DN,
		}
		if entry.Username == "" {
			continue // skip entries without a username
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// syncUsers synchronizes LDAP entries to the local user database.
func (s *Service) syncUsers(ctx context.Context, ldapConfig *ldaps.Ldap, ldapEntries []*ldapEntry, result *ldaps.LdapSyncResult) {
	for _, entry := range ldapEntries {
		result.UsersSynced++

		// Try to find existing user by username in the organization.
		existingUser, err := s.userRepo.GetByUsername(ctx, ldapConfig.OrganizationID, entry.Username)
		if err != nil {
			if isNotFound(err) {
				s.createUserFromLDAP(ctx, ldapConfig, entry, result)
				continue
			}
			// A lookup failure (e.g. database outage) must not fall through to
			// the create branch: that would mask the outage as duplicate-key
			// noise and inflate failure counters with misleading causes.
			slog.ErrorContext(ctx, "failed to look up user for LDAP sync",
				"username", entry.Username,
				"error_category", redact.ErrorCategory(err),
			)
			result.UsersFailed++
			continue
		}

		// Update existing user profile
		changed, err := s.updateUserFromLDAP(ctx, existingUser, entry)
		if err != nil {
			result.UsersFailed++
			continue
		}
		if changed {
			result.UsersUpdated++
		}
	}
}

// createUserFromLDAP creates a new local user from an LDAP entry.
func (s *Service) createUserFromLDAP(ctx context.Context, ldapConfig *ldaps.Ldap, entry *ldapEntry, result *ldaps.LdapSyncResult) {
	id, err := uuid.V7Generator{}.NewString()
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate user ID for LDAP sync", "error", err)
		result.UsersFailed++
		return
	}

	user := &users.User{
		ID:             id,
		OrganizationID: ldapConfig.OrganizationID,
		Username:       entry.Username,
		UserType:       "normal",
		Status:         "active",
		RegisterType:   "ldap",
		RegisterSource: ldapConfig.ID,
		Profile: users.UserProfile{
			DisplayName: entry.DisplayName,
			Email:       entry.Email,
			Phone:       entry.Phone,
			FirstName:   entry.FirstName,
			LastName:    entry.LastName,
		},
		// LDAP users authenticate against the directory and hold no local
		// password hash. Leaving Credentials zero makes the repository skip the
		// credential row, so hasPassword is derived as false on read. A marker
		// such as PasswordAlgo="ldap" is not a hash format and must not be
		// persisted: readers could mistake it for a local credential.
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		// Unique/check violations embed the conflicting directory values in the
		// driver detail, so the message is not logged.
		slog.ErrorContext(ctx, "failed to create user from LDAP sync",
			"username", entry.Username,
			"error_category", redact.ErrorCategory(err),
		)
		result.UsersFailed++
		return
	}

	result.UsersCreated++
}

// updateUserFromLDAP updates an existing user's profile from LDAP data.
// Returns whether any fields were changed, and the persistence error if the
// update failed (callers count that as a sync failure, not a no-op).
func (s *Service) updateUserFromLDAP(ctx context.Context, user *users.User, entry *ldapEntry) (bool, error) {
	changed := false

	if entry.DisplayName != "" && user.Profile.DisplayName != entry.DisplayName {
		user.Profile.DisplayName = entry.DisplayName
		changed = true
	}
	if entry.Email != "" && user.Profile.Email != entry.Email {
		user.Profile.Email = entry.Email
		changed = true
	}
	if entry.Phone != "" && user.Profile.Phone != entry.Phone {
		user.Profile.Phone = entry.Phone
		changed = true
	}
	if entry.FirstName != "" && user.Profile.FirstName != entry.FirstName {
		user.Profile.FirstName = entry.FirstName
		changed = true
	}
	if entry.LastName != "" && user.Profile.LastName != entry.LastName {
		user.Profile.LastName = entry.LastName
		changed = true
	}

	if changed {
		if err := s.userRepo.UpdateProfile(ctx, &user.Profile); err != nil {
			slog.ErrorContext(ctx, "failed to update user profile from LDAP sync",
				"username", entry.Username,
				"error_category", redact.ErrorCategory(err),
			)
			return false, err
		}
	}

	return changed, nil
}

// isNotFound reports whether err carries the platform NotFound code.
func isNotFound(err error) bool {
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}
