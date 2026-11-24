package syncer

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/infra/ldap"
	"github.com/stretchr/testify/assert"
)

func TestLDAPSyncer_ExtractUUID(t *testing.T) {
	t.Run("entryUUID_from_RawAttributes", func(t *testing.T) {
		u := &ldap.LDAPUser{
			DN: "cn=user,dc=example,dc=com",
			RawAttributes: map[string][]string{
				"entryUUID": {"550e8400-e29b-41d4-a716-446655440000"},
			},
		}
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", extractUUID(u))
	})

	t.Run("objectGUID_fallback", func(t *testing.T) {
		u := &ldap.LDAPUser{
			DN: "cn=user,dc=example,dc=com",
			RawAttributes: map[string][]string{
				"objectGUID": {"some-guid-value"},
			},
		}
		assert.Equal(t, "some-guid-value", extractUUID(u))
	})

	t.Run("entryUUID_preferred_over_objectGUID", func(t *testing.T) {
		u := &ldap.LDAPUser{
			DN: "cn=user,dc=example,dc=com",
			RawAttributes: map[string][]string{
				"entryUUID":  {"entry-uuid-value"},
				"objectGUID": {"object-guid-value"},
			},
		}
		assert.Equal(t, "entry-uuid-value", extractUUID(u))
	})

	t.Run("DN_fallback_when_no_RawAttributes", func(t *testing.T) {
		u := &ldap.LDAPUser{
			DN: "cn=user,dc=example,dc=com",
		}
		assert.Equal(t, "cn=user,dc=example,dc=com", extractUUID(u))
	})

	t.Run("empty_entryUUID_uses_objectGUID", func(t *testing.T) {
		u := &ldap.LDAPUser{
			DN: "cn=user,dc=example,dc=com",
			RawAttributes: map[string][]string{
				"entryUUID":  {""},
				"objectGUID": {"guid-value"},
			},
		}
		assert.Equal(t, "guid-value", extractUUID(u))
	})

	t.Run("empty_RawAttributes_uses_DN", func(t *testing.T) {
		u := &ldap.LDAPUser{
			DN:            "cn=user,dc=example,dc=com",
			RawAttributes: map[string][]string{},
		}
		assert.Equal(t, "cn=user,dc=example,dc=com", extractUUID(u))
	})

	t.Run("no_identifiers_returns_empty", func(t *testing.T) {
		u := &ldap.LDAPUser{}
		assert.Equal(t, "", extractUUID(u))
	})
}

func TestLDAPSyncer_MapLDAPUser(t *testing.T) {
	s := &LDAPSyncer{}

	t.Run("default_mapping", func(t *testing.T) {
		u := &ldap.LDAPUser{
			UID:            "jdoe",
			CommonName:     "John Doe",
			Surname:        "Doe",
			GivenName:      "John",
			Email:          "jdoe@example.com",
			Mobile:         "555-1234",
			Title:          "Engineer",
			Department:     "Engineering",
			EmployeeNumber: "E001",
		}
		result := s.mapLDAPUser(u, nil)
		assert.Equal(t, "jdoe", result["username"])
		assert.Equal(t, "John Doe", result["displayName"])
		assert.Equal(t, "Doe", result["familyName"])
		assert.Equal(t, "John", result["givenName"])
		assert.Equal(t, "jdoe@example.com", result["email"])
		assert.Equal(t, "555-1234", result["phone"])
		assert.Equal(t, "Engineer", result["title"])
		assert.Equal(t, "Engineering", result["department"])
		assert.Equal(t, "E001", result["employeeNumber"])
	})

	t.Run("default_mapping_empty_map", func(t *testing.T) {
		u := &ldap.LDAPUser{
			UID:        "jdoe",
			CommonName: "John Doe",
			Email:      "jdoe@example.com",
		}
		result := s.mapLDAPUser(u, map[string]string{})
		assert.Equal(t, "jdoe", result["username"])
		assert.Equal(t, "John Doe", result["displayName"])
		assert.Equal(t, "jdoe@example.com", result["email"])
	})

	t.Run("custom_attribute_mapping", func(t *testing.T) {
		u := &ldap.LDAPUser{
			UID:        "jdoe",
			CommonName: "John Doe",
			Email:      "jdoe@example.com",
			Mobile:     "555-1234",
		}
		attributeMap := map[string]string{
			"uid":    "loginName",
			"cn":     "fullName",
			"mail":   "emailAddress",
			"mobile": "phoneNumber",
		}
		result := s.mapLDAPUser(u, attributeMap)
		assert.Equal(t, "jdoe", result["loginName"])
		assert.Equal(t, "John Doe", result["fullName"])
		assert.Equal(t, "jdoe@example.com", result["emailAddress"])
		assert.Equal(t, "555-1234", result["phoneNumber"])
	})

	t.Run("partial_mapping_keeps_unmapped", func(t *testing.T) {
		u := &ldap.LDAPUser{
			UID:        "jdoe",
			CommonName: "John Doe",
			Surname:    "Doe",
		}
		attributeMap := map[string]string{
			"uid": "loginName",
		}
		result := s.mapLDAPUser(u, attributeMap)
		assert.Equal(t, "jdoe", result["loginName"])
		// sn is not in attributeMap, so it's not mapped
		_, hasSurname := result["familyName"]
		assert.False(t, hasSurname)
	})
}

func TestLDAPSyncer_MappedUserToMap(t *testing.T) {
	s := &LDAPSyncer{}

	user := &LDAPMappedUser{
		Username:       "jdoe",
		DisplayName:    "John Doe",
		Email:          "jdoe@example.com",
		Phone:          "555-1234",
		GivenName:      "John",
		Surname:        "Doe",
		Title:          "Engineer",
		Department:     "Engineering",
		EmployeeNumber: "E001",
		UUID:           "uuid-123",
		DN:             "cn=jdoe,dc=example,dc=com",
		Disabled:       false,
	}

	result := s.mappedUserToMap(user)
	assert.Equal(t, "jdoe", result["username"])
	assert.Equal(t, "John Doe", result["displayName"])
	assert.Equal(t, "jdoe@example.com", result["email"])
	assert.Equal(t, "555-1234", result["phone"])
	assert.Equal(t, "John", result["givenName"])
	assert.Equal(t, "Doe", result["familyName"])
	assert.Equal(t, "Engineer", result["title"])
	assert.Equal(t, "Engineering", result["department"])
	assert.Equal(t, "E001", result["employeeNumber"])
	assert.Equal(t, "uuid-123", result["uuid"])
	assert.Equal(t, "cn=jdoe,dc=example,dc=com", result["dn"])
	assert.Equal(t, false, result["disabled"])
}

func TestLDAPSyncer_MappedGroupToMap(t *testing.T) {
	s := &LDAPSyncer{}

	group := &LDAPMappedGroup{
		Name:        "engineers",
		DisplayName: "Engineering Team",
		Description: "All engineers",
		MemberDNs:   []string{"cn=user1,dc=example,dc=com", "cn=user2,dc=example,dc=com"},
		MemberUIDs:  []string{"user1", "user2"},
		DN:          "cn=engineers,dc=example,dc=com",
	}

	result := s.mappedGroupToMap(group)
	assert.Equal(t, "engineers", result["name"])
	assert.Equal(t, "Engineering Team", result["displayName"])
	assert.Equal(t, "All engineers", result["description"])
	assert.Equal(t, []string{"cn=user1,dc=example,dc=com", "cn=user2,dc=example,dc=com"}, result["memberDNs"])
	assert.Equal(t, []string{"user1", "user2"}, result["memberUIDs"])
	assert.Equal(t, "cn=engineers,dc=example,dc=com", result["dn"])
}

func TestNewLDAPSyncer(t *testing.T) {
	s := NewLDAPSyncer()
	assert.NotNil(t, s)
	assert.NotNil(t, s.client)
}
