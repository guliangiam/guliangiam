package syncer

import (
	"context"
	"fmt"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domainldaps "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/infra/ldap"
)

// LDAPPreviewUser represents an LDAP user with local existence info for preview.
type LDAPPreviewUser struct {
	UUID        string `json:"uuid"`
	UID         string `json:"uid"`
	CN          string `json:"cn"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	GivenName   string `json:"givenName"`
	Surname     string `json:"surname"`
	Title       string `json:"title"`
	Department  string `json:"department"`
	DN          string `json:"dn"`
	ExistsLocal bool   `json:"existsLocal"`
}

// LDAPMappedUser is a user fully mapped from LDAP attributes, ready for upsert.
type LDAPMappedUser struct {
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	GivenName      string `json:"givenName"`
	Surname        string `json:"surname"`
	Title          string `json:"title"`
	Department     string `json:"department"`
	EmployeeNumber string `json:"employeeNumber"`
	UUID           string `json:"uuid"` // LDAP entryUUID for identity linking
	DN             string `json:"dn"`
	Disabled       bool   `json:"disabled"`
}

// LDAPMappedGroup is a group fully mapped from LDAP attributes, ready for upsert.
type LDAPMappedGroup struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	MemberDNs   []string `json:"memberDNs"`
	MemberUIDs  []string `json:"memberUIDs"`
	DN          string   `json:"dn"`
}

// LDAPSyncer syncs users and groups from an LDAP directory.
// It uses the infra/ldap client for actual LDAP communication.
type LDAPSyncer struct {
	client *ldap.Client
}

// NewLDAPSyncer creates a new LDAP syncer with the default LDAP client.
func NewLDAPSyncer() *LDAPSyncer {
	return &LDAPSyncer{
		client: ldap.NewClient(),
	}
}

// PreviewUsers searches LDAP and returns a preview user list without writing to the database.
// The existsLocal field should be populated by the caller after checking against local users.
func (s *LDAPSyncer) PreviewUsers(ctx context.Context, config *SyncerConfig) ([]*LDAPPreviewUser, error) {
	useTLS := ldap.ShouldUseTLS(config.Port)
	conn, err := s.client.Connect(config.Host, config.Port, config.Username, config.Password, useTLS)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "ldap preview connect")
	}
	defer conn.Close()

	filter := config.Filter
	if filter == "" {
		filter = ldap.DefaultUserFilter("")
	}

	baseDN := config.BaseDN
	if baseDN == "" {
		return nil, apperrors.NewInvalidArgument("ldap preview: BaseDN is required")
	}

	// Request entryUUID attribute for identity tracking
	attrs := []string{
		"dn", "uid", "cn", "sn", dirAttrGivenName, "mail",
		"mobile", dirAttrTitle, "departmentNumber", dirAttrEmployeeNumber,
		"entryUUID", "objectGUID",
	}

	users, err := s.client.SearchUsers(conn, baseDN, filter, attrs)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "ldap preview search users")
	}

	preview := make([]*LDAPPreviewUser, 0, len(users))
	for _, u := range users {
		uuid := extractUUID(u)
		preview = append(preview, &LDAPPreviewUser{
			UUID:       uuid,
			UID:        u.UID,
			CN:         u.CommonName,
			Email:      u.Email,
			Phone:      u.Mobile,
			GivenName:  u.GivenName,
			Surname:    u.Surname,
			Title:      u.Title,
			Department: u.Department,
			DN:         u.DN,
		})
	}

	return preview, nil
}

// GetMappedUsers returns fully mapped users ready for database upsert.
// If filterUUIDs is non-empty, only users matching those UUIDs are returned.
func (s *LDAPSyncer) GetMappedUsers(ctx context.Context, config *SyncerConfig, filterUUIDs []string) ([]*LDAPMappedUser, error) {
	useTLS := ldap.ShouldUseTLS(config.Port)
	conn, err := s.client.Connect(config.Host, config.Port, config.Username, config.Password, useTLS)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "ldap mapped connect")
	}
	defer conn.Close()

	filter := config.Filter
	if filter == "" {
		filter = ldap.DefaultUserFilter("")
	}

	baseDN := config.BaseDN
	if baseDN == "" {
		return nil, apperrors.NewInvalidArgument("ldap mapped: BaseDN is required")
	}

	attrs := []string{
		"dn", "uid", "cn", "sn", dirAttrGivenName, "mail",
		"mobile", dirAttrTitle, "departmentNumber", dirAttrEmployeeNumber,
		"entryUUID", "objectGUID", "userAccountControl",
	}

	users, err := s.client.SearchUsers(conn, baseDN, filter, attrs)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "ldap mapped search users")
	}

	uuidFilter := make(map[string]bool, len(filterUUIDs))
	for _, id := range filterUUIDs {
		uuidFilter[id] = true
	}

	mapped := make([]*LDAPMappedUser, 0, len(users))
	for _, u := range users {
		uuid := extractUUID(u)
		if uuid == "" {
			continue
		}

		// Apply UUID filter if specified
		if len(uuidFilter) > 0 && !uuidFilter[uuid] {
			continue
		}

		fieldMap := s.mapLDAPUser(u, config.AttributeMap)
		mappedUser := &LDAPMappedUser{
			Username:       fieldMap["username"],
			DisplayName:    fieldMap["displayName"],
			Email:          fieldMap["email"],
			Phone:          fieldMap["phone"],
			GivenName:      fieldMap[dirAttrGivenName],
			Surname:        fieldMap["familyName"],
			Title:          fieldMap[dirAttrTitle],
			Department:     fieldMap["department"],
			EmployeeNumber: fieldMap[dirAttrEmployeeNumber],
			UUID:           uuid,
			DN:             u.DN,
			Disabled:       u.Disabled,
		}

		if mappedUser.Username == "" {
			mappedUser.Username = u.UID
		}

		mapped = append(mapped, mappedUser)
	}

	return mapped, nil
}

// GetMappedGroups returns fully mapped groups ready for database upsert.
func (s *LDAPSyncer) GetMappedGroups(ctx context.Context, config *SyncerConfig) ([]*LDAPMappedGroup, error) {
	useTLS := ldap.ShouldUseTLS(config.Port)
	conn, err := s.client.Connect(config.Host, config.Port, config.Username, config.Password, useTLS)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "ldap mapped groups connect")
	}
	defer conn.Close()

	groupFilter := "(objectClass=groupOfNames)"
	if config.Filter != "" {
		groupFilter = config.Filter
	}

	baseDN := config.BaseDN
	if baseDN == "" {
		return nil, apperrors.NewInvalidArgument("ldap mapped groups: BaseDN is required")
	}

	groups, err := s.client.SearchGroups(conn, baseDN, groupFilter, nil)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "ldap mapped search groups")
	}

	mapped := make([]*LDAPMappedGroup, 0, len(groups))
	for _, g := range groups {
		if g.CommonName == "" {
			continue
		}
		mapped = append(mapped, &LDAPMappedGroup{
			Name:        g.CommonName,
			DisplayName: g.CommonName,
			Description: g.Description,
			MemberDNs:   g.MemberDNs,
			MemberUIDs:  g.MemberUIDs,
			DN:          g.DN,
		})
	}

	return mapped, nil
}

// SyncUsers synchronizes users from the LDAP directory.
// This implements the Engine interface. It returns a SyncResult with counts.
// When OnUserSynced callback is set, it persists each mapped user; otherwise counts only.
func (s *LDAPSyncer) SyncUsers(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	mappedUsers, err := s.GetMappedUsers(ctx, config, nil)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{
		TotalUsers: len(mappedUsers),
	}

	for _, u := range mappedUsers {
		if u.Username == "" {
			result.FailedUsers++
			result.Errors = append(result.Errors, fmt.Sprintf("ldap user missing uid: dn=%s", u.DN))
			continue
		}

		if config.OnUserSynced != nil {
			mappedMap := s.mappedUserToMap(u)
			created, err := config.OnUserSynced(ctx, "ldap", u.UUID, mappedMap)
			if err != nil {
				result.FailedUsers++
				result.Errors = append(result.Errors, fmt.Sprintf("ldap user write-back failed: uid=%s, err=%v", u.Username, err))
				continue
			}
			if created {
				result.AddedUsers++
			} else {
				result.UpdatedUsers++
			}
		} else {
			// Read-only mode: count without persisting
			result.AddedUsers++
		}
	}

	return result, nil
}

// SyncGroups synchronizes groups from the LDAP directory.
// This implements the Engine interface. When OnGroupSynced callback is set,
// it persists each mapped group; otherwise counts only.
func (s *LDAPSyncer) SyncGroups(ctx context.Context, config *SyncerConfig) (*SyncResult, error) {
	mappedGroups, err := s.GetMappedGroups(ctx, config)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{
		TotalGroups: len(mappedGroups),
	}

	for _, g := range mappedGroups {
		if config.OnGroupSynced != nil {
			mappedMap := s.mappedGroupToMap(g)
			created, err := config.OnGroupSynced(ctx, config.OrganizationID, g.DN, mappedMap)
			if err != nil {
				result.FailedGroups++
				result.Errors = append(result.Errors, fmt.Sprintf("ldap group write-back failed: name=%s, err=%v", g.Name, err))
				continue
			}
			if created {
				result.AddedGroups++
			} else {
				result.UpdatedGroups++
			}
		} else {
			// Read-only mode: count without persisting
			result.AddedGroups++
		}
	}

	return result, nil
}

// TestConnection tests the LDAP connection.
func (s *LDAPSyncer) TestConnection(ctx context.Context, config *SyncerConfig) error {
	useTLS := ldap.ShouldUseTLS(config.Port)
	return s.client.TestConnection(config.Host, config.Port, config.Username, config.Password, useTLS)
}

// mapLDAPUser maps an LDAP user to a field map using the configured attribute mapping.
// Default mapping (when AttributeMap is empty):
//
//	uid -> username, cn -> displayName, sn -> familyName,
//	givenName -> givenName, mail -> email, mobile -> phone
func (s *LDAPSyncer) mapLDAPUser(u *ldap.LDAPUser, attributeMap map[string]string) map[string]string {
	result := make(map[string]string)

	if len(attributeMap) == 0 {
		result["username"] = u.UID
		result["displayName"] = u.CommonName
		result["familyName"] = u.Surname
		result[dirAttrGivenName] = u.GivenName
		result["email"] = u.Email
		result["phone"] = u.Mobile
		result[dirAttrTitle] = u.Title
		result["department"] = u.Department
		result[dirAttrEmployeeNumber] = u.EmployeeNumber
		return result
	}

	if target, ok := attributeMap["uid"]; ok {
		result[target] = u.UID
	}
	if target, ok := attributeMap["cn"]; ok {
		result[target] = u.CommonName
	}
	if target, ok := attributeMap["sn"]; ok {
		result[target] = u.Surname
	}
	if target, ok := attributeMap[dirAttrGivenName]; ok {
		result[target] = u.GivenName
	}
	if target, ok := attributeMap["mail"]; ok {
		result[target] = u.Email
	}
	if target, ok := attributeMap["mobile"]; ok {
		result[target] = u.Mobile
	}
	if target, ok := attributeMap["telephoneNumber"]; ok {
		result[target] = u.Telephone
	}
	if target, ok := attributeMap[dirAttrTitle]; ok {
		result[target] = u.Title
	}
	if target, ok := attributeMap["departmentNumber"]; ok {
		result[target] = u.Department
	}
	if target, ok := attributeMap[dirAttrEmployeeNumber]; ok {
		result[target] = u.EmployeeNumber
	}

	return result
}

// extractUUID extracts the unique identifier from an LDAP user entry.
// It tries entryUUID (OpenLDAP) first, then objectGUID (Active Directory),
// then falls back to the DN.
func extractUUID(u *ldap.LDAPUser) string {
	if u.RawAttributes != nil {
		if uuids, ok := u.RawAttributes["entryUUID"]; ok && len(uuids) > 0 && uuids[0] != "" {
			return uuids[0]
		}
		if guids, ok := u.RawAttributes["objectGUID"]; ok && len(guids) > 0 && guids[0] != "" {
			return guids[0]
		}
	}
	// Fall back to DN as a stable identifier
	if u.DN != "" {
		return u.DN
	}
	return ""
}

// mappedUserToMap converts an LDAPMappedUser to a map for the write-back callback.
func (s *LDAPSyncer) mappedUserToMap(u *LDAPMappedUser) map[string]interface{} {
	return map[string]interface{}{
		"username":            u.Username,
		"displayName":         u.DisplayName,
		"email":               u.Email,
		"phone":               u.Phone,
		dirAttrGivenName:      u.GivenName,
		"familyName":          u.Surname,
		dirAttrTitle:          u.Title,
		"department":          u.Department,
		dirAttrEmployeeNumber: u.EmployeeNumber,
		"uuid":                u.UUID,
		"dn":                  u.DN,
		"disabled":            u.Disabled,
	}
}

// mappedGroupToMap converts an LDAPMappedGroup to a map for the write-back callback.
func (s *LDAPSyncer) mappedGroupToMap(g *LDAPMappedGroup) map[string]interface{} {
	return map[string]interface{}{
		"name":        g.Name,
		"displayName": g.DisplayName,
		"description": g.Description,
		"memberDNs":   g.MemberDNs,
		"memberUIDs":  g.MemberUIDs,
		"dn":          g.DN,
	}
}

var _ Engine = (*LDAPSyncer)(nil)
var _ domainldaps.LDAPSyncer = (*LDAPSyncer)(nil)

func (s *LDAPSyncer) PreviewUsersDomain(ctx context.Context, config *domainldaps.SyncerConfig) ([]*domainldaps.LDAPPreviewUser, error) {
	sc := toInfraSyncerConfig(config)
	users, err := s.PreviewUsers(ctx, sc)
	if err != nil {
		return nil, err
	}
	result := make([]*domainldaps.LDAPPreviewUser, len(users))
	for i, u := range users {
		result[i] = &domainldaps.LDAPPreviewUser{
			UUID:        u.UUID,
			Username:    u.UID,
			DisplayName: u.CN,
			Email:       u.Email,
			ExistsLocal: u.ExistsLocal,
		}
	}
	return result, nil
}

func (s *LDAPSyncer) GetMappedUsersDomain(ctx context.Context, config *domainldaps.SyncerConfig, uuids []string) ([]*domainldaps.LDAPMappedUser, error) {
	sc := toInfraSyncerConfig(config)
	users, err := s.GetMappedUsers(ctx, sc, uuids)
	if err != nil {
		return nil, err
	}
	result := make([]*domainldaps.LDAPMappedUser, len(users))
	for i, u := range users {
		result[i] = &domainldaps.LDAPMappedUser{
			UUID:           u.UUID,
			Username:       u.Username,
			DisplayName:    u.DisplayName,
			Email:          u.Email,
			Phone:          u.Phone,
			GivenName:      u.GivenName,
			Surname:        u.Surname,
			Title:          u.Title,
			Department:     u.Department,
			EmployeeNumber: u.EmployeeNumber,
			DN:             u.DN,
			Disabled:       u.Disabled,
		}
	}
	return result, nil
}

func (s *LDAPSyncer) GetMappedGroupsDomain(ctx context.Context, config *domainldaps.SyncerConfig) ([]*domainldaps.LDAPMappedGroup, error) {
	sc := toInfraSyncerConfig(config)
	groups, err := s.GetMappedGroups(ctx, sc)
	if err != nil {
		return nil, err
	}
	result := make([]*domainldaps.LDAPMappedGroup, len(groups))
	for i, g := range groups {
		result[i] = &domainldaps.LDAPMappedGroup{
			Name:        g.Name,
			DisplayName: g.DisplayName,
		}
	}
	return result, nil
}

func toInfraSyncerConfig(dc *domainldaps.SyncerConfig) *SyncerConfig {
	return &SyncerConfig{
		ProviderType:   dc.ProviderType,
		Host:           dc.Host,
		Port:           dc.Port,
		Username:       dc.Username,
		Password:       dc.Password,
		BaseDN:         dc.BaseDN,
		Filter:         dc.Filter,
		OrganizationID: dc.OrganizationID,
		SyncInterval:   dc.SyncInterval,
	}
}
