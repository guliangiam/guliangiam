package syncer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOktaSyncer_OktaBaseURL(t *testing.T) {
	s := &OktaSyncer{}

	tests := []struct {
		name     string
		config   *SyncerConfig
		expected string
	}{
		{"api_url", &SyncerConfig{APIURL: "https://example.okta.com/api/v1"}, "https://example.okta.com/api/v1"},
		{"host", &SyncerConfig{Host: "example.okta.com"}, "https://example.okta.com"},
		{"host_with_scheme", &SyncerConfig{Host: "https://example.okta.com"}, "https://example.okta.com"},
		{"empty", &SyncerConfig{}, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := s.oktaBaseURL(tc.config)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestOktaSyncer_GetAccessToken_SSWS(t *testing.T) {
	s := &OktaSyncer{}
	token, err := s.getOktaAccessToken(t.Context(), &SyncerConfig{ClientSecret: "my-ssws-token"})
	assert.NoError(t, err)
	assert.Equal(t, "my-ssws-token", token)
}

func TestOktaSyncer_GetAccessToken_MissingConfig(t *testing.T) {
	s := &OktaSyncer{}
	_, err := s.getOktaAccessToken(t.Context(), &SyncerConfig{})
	assert.Error(t, err)
}

func TestOktaSyncer_ParseOktaNextLink(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"next_link", `<https://example.okta.com/api/v1/users?after=xxx>; rel="next"`, "https://example.okta.com/api/v1/users?after=xxx"},
		{"no_next", `<https://example.okta.com/api/v1/users>; rel="prev"`, ""},
		{"multiple_links", `<https://example.okta.com/api/v1/users>; rel="prev", <https://example.okta.com/api/v1/users?after=yyy>; rel="next"`, "https://example.okta.com/api/v1/users"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseOktaNextLink(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestOktaSyncer_MapUser_Default(t *testing.T) {
	s := &OktaSyncer{}
	item := oktaUserResponse{
		ID:     "user-1",
		Status: "ACTIVE",
	}
	item.Profile.Login = "jdoe"
	item.Profile.FirstName = "John"
	item.Profile.LastName = "Doe"
	item.Profile.Email = "jdoe@example.com"

	result := s.mapUser(&item, nil)
	assert.Equal(t, "user-1", result["id"])
	assert.Equal(t, "jdoe", result["username"])
	assert.Equal(t, "John Doe", result["displayName"])
	assert.Equal(t, "jdoe@example.com", result["email"])
	assert.Equal(t, "ACTIVE", result["status"])
}

func TestOktaSyncer_MapUser_CustomMapping(t *testing.T) {
	s := &OktaSyncer{}
	item := oktaUserResponse{
		ID:     "user-1",
		Status: "ACTIVE",
	}
	item.Profile.Login = "jdoe"
	item.Profile.Email = "jdoe@example.com"

	attributeMap := map[string]string{
		"id":            "externalId",
		"profile.login": "loginName",
		"profile.email": "emailAddress",
	}

	result := s.mapUser(&item, attributeMap)
	assert.Equal(t, "user-1", result["externalId"])
	assert.Equal(t, "jdoe", result["loginName"])
	assert.Equal(t, "jdoe@example.com", result["emailAddress"])
}

func TestOktaSyncer_MapGroup_Default(t *testing.T) {
	s := &OktaSyncer{}
	item := oktaGroupResponse{
		ID:   "group-1",
		Type: "OKTA_GROUP",
	}
	item.Profile.Name = "Admins"
	item.Profile.Description = "Admin group"

	result := s.mapGroup(item, nil)
	assert.Equal(t, "group-1", result["id"])
	assert.Equal(t, "OKTA_GROUP", result["type"])
	assert.Equal(t, "Admins", result["name"])
	assert.Equal(t, "Admin group", result["description"])
}

func TestOktaSyncer_MapGroup_CustomMapping(t *testing.T) {
	s := &OktaSyncer{}
	item := oktaGroupResponse{ID: "group-1", Type: "OKTA_GROUP"}
	item.Profile.Name = "Admins"

	attributeMap := map[string]string{
		"id":           "groupId",
		"profile.name": "groupName",
	}

	result := s.mapGroup(item, attributeMap)
	assert.Equal(t, "group-1", result["groupId"])
	assert.Equal(t, "Admins", result["groupName"])
}

func TestIndexByte(t *testing.T) {
	assert.Equal(t, 0, indexByte("abc", 'a'))
	assert.Equal(t, 2, indexByte("abc", 'c'))
	assert.Equal(t, -1, indexByte("abc", 'd'))
	assert.Equal(t, -1, indexByte("", 'a'))
}

func TestContainsStr(t *testing.T) {
	assert.True(t, containsStr("hello world", "world"))
	assert.True(t, containsStr("hello", "hello"))
	assert.False(t, containsStr("hello", "world"))
	assert.False(t, containsStr("hi", "hello"))
}
