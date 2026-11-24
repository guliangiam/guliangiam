package ldap

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/go-ldap/ldap/v3"
	domainldaps "github.com/guliangiam/guliangiam/internal/domain/ldaps"
)

// ldapOperationTimeout bounds every LDAP read/write so an unresponsive
// directory surfaces as a delivery error instead of a stalled request.
// ldap.Conn.SetTimeout takes a time.Duration: 30*time.Second, not 30*1000
// (which would be 30µs).
const ldapOperationTimeout = 30 * time.Second

// ldapDialTimeout bounds the TCP/TLS handshake. SetTimeout only applies after
// the connection is established, so an unreachable host would otherwise hang
// until the OS-level dial timeout (minutes).
const ldapDialTimeout = 10 * time.Second

// ldapSearchSizeLimit caps a single directory search so a misconfigured filter
// cannot pull an unbounded result set into memory.
const ldapSearchSizeLimit = 10000

// Client is a reusable LDAP client for connecting to and querying LDAP directories.
type Client struct{}

// NewClient creates a new LDAP client.
func NewClient() *Client {
	return &Client{}
}

// ldapURL builds the connection URL used by ldap.DialURL.
// It keeps the previous "host:port" semantics while selecting the LDAPS
// scheme when TLS is requested.
func ldapURL(host string, port int, useTLS bool) string {
	scheme := "ldap"
	if useTLS {
		scheme = "ldaps"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, port)
}

// Connect establishes a connection to the LDAP server and authenticates.
// host: LDAP server hostname (e.g., "ldap.example.com")
// port: LDAP server port (389 for LDAP, 636 for LDAPS)
// bindDN: Distinguished Name for binding (e.g., "cn=admin,dc=example,dc=com")
// bindPassword: Password for the bind DN
// useTLS: Whether to use TLS (LDAPS)
func (c *Client) Connect(host string, port int, bindDN, bindPassword string, useTLS bool) (*ldap.Conn, error) {
	url := ldapURL(host, port, useTLS)

	var conn *ldap.Conn
	var err error

	dialer := ldap.DialWithDialer(&net.Dialer{Timeout: ldapDialTimeout})
	if useTLS {
		conn, err = ldap.DialURL(url, dialer, ldap.DialWithTLSConfig(&tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}))
	} else {
		conn, err = ldap.DialURL(url, dialer)
	}

	if err != nil {
		return nil, apperrors.WrapInternalf(err, "ldap connect to %s", url)
	}

	conn.SetTimeout(ldapOperationTimeout)

	// Authenticate
	if err := conn.Bind(bindDN, bindPassword); err != nil {
		conn.Close()
		return nil, apperrors.WrapInternalf(err, "ldap bind")
	}

	return conn, nil
}

// TestConnection tests the LDAP connection by connecting and binding.
func (c *Client) TestConnection(host string, port int, bindDN, bindPassword string, useTLS bool) error {
	conn, err := c.Connect(host, port, bindDN, bindPassword, useTLS)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// LDAPUser represents a user entry from LDAP.
type LDAPUser struct {
	DN             string
	UID            string
	CommonName     string
	Surname        string
	GivenName      string
	Email          string
	Telephone      string
	Mobile         string
	Title          string
	Department     string
	EmployeeNumber string
	Disabled       bool
	RawAttributes  map[string][]string
}

// LDAPGroup represents a group entry from LDAP.
type LDAPGroup struct {
	DN            string
	CommonName    string
	Description   string
	MemberDNs     []string
	MemberUIDs    []string
	RawAttributes map[string][]string
}

// SearchUsers searches for user entries in LDAP.
// baseDN: The base DN to search from (e.g., "ou=users,dc=example,dc=com")
// filter: LDAP search filter (e.g., "(objectClass=person)" or "(objectClass=user)")
// attributes: LDAP attributes to retrieve (nil means all user-related attributes)
// The default attribute set maps onto RFC 4519 user attributes
// (uid=USERID, cn=commonName, sn=surname, givenName=givenName,
// mail=e-mailAddress, telephoneNumber=telephoneNumber, title=title).
// mobile/departmentNumber/employeeNumber are not in RFC 4519 core but are
// the de-facto OpenLDAP inetOrgPerson / AD extension names.
// userAccountControl is the AD-only attribute (see below).
func (c *Client) SearchUsers(conn *ldap.Conn, baseDN, filter string, attributes []string) ([]*LDAPUser, error) {
	if attributes == nil {
		attributes = []string{
			"dn", "uid", "cn", "sn", "givenName", "mail",
			"telephoneNumber", "mobile", "title", "departmentNumber",
			"employeeNumber", "userAccountControl",
		}
	}

	searchReq := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		ldapSearchSizeLimit, 0, false,
		filter,
		attributes,
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "ldap search users")
	}

	users := make([]*LDAPUser, 0, len(sr.Entries))
	for _, entry := range sr.Entries {
		user := &LDAPUser{
			DN:             entry.DN,
			UID:            entry.GetAttributeValue("uid"),
			CommonName:     entry.GetAttributeValue("cn"),
			Surname:        entry.GetAttributeValue("sn"),
			GivenName:      entry.GetAttributeValue("givenName"),
			Email:          entry.GetAttributeValue("mail"),
			Telephone:      entry.GetAttributeValue("telephoneNumber"),
			Mobile:         entry.GetAttributeValue("mobile"),
			Title:          entry.GetAttributeValue("title"),
			Department:     entry.GetAttributeValue("departmentNumber"),
			EmployeeNumber: entry.GetAttributeValue("employeeNumber"),
			RawAttributes:  attrsToMap(entry.Attributes),
		}

		// Active Directory only: userAccountControl bit 0x2 is
		// UF_ACCOUNTDISABLE (Microsoft Learn, AD attribute schema).
		if uac := entry.GetAttributeValue("userAccountControl"); uac != "" {
			var uacVal int
			if _, err := fmt.Sscanf(uac, "%d", &uacVal); err == nil {
				user.Disabled = (uacVal & 2) != 0 // UF_ACCOUNTDISABLE bit
			}
		}

		users = append(users, user)
	}

	return users, nil
}

// SearchGroups searches for group entries in LDAP.
// baseDN: The base DN to search from (e.g., "ou=groups,dc=example,dc=com")
// filter: LDAP search filter (e.g., "(objectClass=group)" or "(objectClass=groupOfNames)")
// attributes: LDAP attributes to retrieve (nil means all group-related attributes)
func (c *Client) SearchGroups(conn *ldap.Conn, baseDN, filter string, attributes []string) ([]*LDAPGroup, error) {
	if attributes == nil {
		attributes = []string{
			"dn", "cn", "description", "member", "memberUid",
		}
	}

	searchReq := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		ldapSearchSizeLimit, 0, false,
		filter,
		attributes,
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "ldap search groups")
	}

	groups := make([]*LDAPGroup, 0, len(sr.Entries))
	for _, entry := range sr.Entries {
		group := &LDAPGroup{
			DN:            entry.DN,
			CommonName:    entry.GetAttributeValue("cn"),
			Description:   entry.GetAttributeValue("description"),
			MemberDNs:     entry.GetAttributeValues("member"),
			MemberUIDs:    entry.GetAttributeValues("memberUid"),
			RawAttributes: attrsToMap(entry.Attributes),
		}

		groups = append(groups, group)
	}

	return groups, nil
}

// ShouldUseTLS determines if TLS should be used based on the port.
// 636 is the LDAPS well-known port (RFC 4511 / IANA service names);
// 3269 is the Microsoft AD Global Catalog over LDAPS port.
// Plain 389/3268 stay unencrypted (StartTLS on 389 is a future form,
// see OPEN-API-75 P9 adjudication for FreeIPA/389DS reuse).
func ShouldUseTLS(port int) bool {
	return port == 636 || port == 3269
}

// DefaultUserFilter returns a reasonable default LDAP filter for users based on the server type.
// For Active Directory, use "(objectClass=user)".
// For OpenLDAP and others, use "(objectClass=person)".
func DefaultUserFilter(serverType string) string {
	switch strings.ToLower(serverType) {
	case "activedirectory", "ad":
		return "(objectClass=user)"
	default:
		return "(objectClass=person)"
	}
}

// DefaultGroupFilter returns a reasonable default LDAP filter for groups.
func DefaultGroupFilter(serverType string) string {
	switch strings.ToLower(serverType) {
	case "activedirectory", "ad":
		return "(objectClass=group)"
	default:
		return "(objectClass=groupOfNames)"
	}
}

func attrsToMap(attrs []*ldap.EntryAttribute) map[string][]string {
	m := make(map[string][]string, len(attrs))
	for _, a := range attrs {
		m[a.Name] = a.Values
	}
	return m
}

var _ domainldaps.LDAPTestClient = (*Client)(nil)
