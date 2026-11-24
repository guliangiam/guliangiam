package scim

import (
	"testing"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/groups"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToScimUser(t *testing.T) {
	now := time.Now()
	u := &users.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		IsForbidden:    false,
		ExternalID:     "ext-1",
		Profile: users.UserProfile{
			DisplayName: "Alice Smith",
			FirstName:   "Alice",
			LastName:    "Smith",
			Email:       "alice@example.com",
			Phone:       "+1234567890",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	su := toScimUser(u)
	assert.Equal(t, SchemaUser, su.Schemas[0])
	assert.Equal(t, "user-1", su.ID)
	assert.Equal(t, "alice", su.UserName)
	assert.Equal(t, "Alice Smith", su.DisplayName)
	assert.True(t, su.Active)
	assert.Equal(t, "ext-1", su.ExternalID)
	assert.NotNil(t, su.Name)
	assert.Equal(t, "Alice", su.Name.GivenName)
	assert.Equal(t, "Smith", su.Name.FamilyName)
	assert.Len(t, su.Emails, 1)
	assert.Equal(t, "alice@example.com", su.Emails[0].Value)
	assert.Len(t, su.PhoneNumbers, 1)
	assert.Equal(t, "+1234567890", su.PhoneNumbers[0].Value)
}

func TestToScimUser_FallbackDisplayName(t *testing.T) {
	u := &users.User{
		Username: "bob",
		Profile:  users.UserProfile{},
	}
	su := toScimUser(u)
	assert.Equal(t, "bob", su.DisplayName) // falls back to username
}

func TestToScimUser_ForbiddenUser(t *testing.T) {
	u := &users.User{
		Username:    "bob",
		IsForbidden: true,
	}
	su := toScimUser(u)
	assert.False(t, su.Active)
}

func TestToScimUser_NoEmail(t *testing.T) {
	u := &users.User{Username: "bob"}
	su := toScimUser(u)
	assert.Nil(t, su.Emails)
}

func TestToScimUser_NoPhone(t *testing.T) {
	u := &users.User{Username: "bob"}
	su := toScimUser(u)
	assert.Nil(t, su.PhoneNumbers)
}

func TestToScimGroup(t *testing.T) {
	now := time.Now()
	g := &groups.Group{
		ID:          "group-1",
		DisplayName: "Admins",
		Users:       []string{"user-1", "user-2"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	sg := toScimGroup(g)
	assert.Equal(t, SchemaGroup, sg.Schemas[0])
	assert.Equal(t, "group-1", sg.ID)
	assert.Equal(t, "Admins", sg.DisplayName)
	assert.Len(t, sg.Members, 2)
	assert.Equal(t, "user-1", sg.Members[0].Value)
	assert.Equal(t, "user-2", sg.Members[1].Value)
}

func TestToScimGroup_NoMembers(t *testing.T) {
	g := &groups.Group{ID: "group-1", DisplayName: "Empty"}
	sg := toScimGroup(g)
	assert.Nil(t, sg.Members)
}

func TestApplyUserFilterAST_Nil(t *testing.T) {
	query := &users.ListQuery{}
	applyUserFilterAST(nil, query)
	assert.Empty(t, query.Keyword)
}

func TestApplyUserFilterAST_UsernameEq(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterEq, AttrPath: "userName", Value: "alice"}
	applyUserFilterAST(node, query)
	assert.Equal(t, "username", query.SearchField)
	assert.Equal(t, "alice", query.SearchValue)
}

func TestApplyUserFilterAST_UsernameSw(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterSw, AttrPath: "userName", Value: "ali"}
	applyUserFilterAST(node, query)
	assert.Equal(t, "ali", query.Keyword)
}

func TestApplyUserFilterAST_EmailEq(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterEq, AttrPath: "emails.value", Value: "alice@example.com"}
	applyUserFilterAST(node, query)
	assert.Equal(t, "email", query.SearchField)
	assert.Equal(t, "alice@example.com", query.SearchValue)
}

func TestApplyUserFilterAST_ActiveEq(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterEq, AttrPath: "active", Value: "true"}
	applyUserFilterAST(node, query)
	assert.Equal(t, "active", query.Status)
}

func TestApplyUserFilterAST_ActiveNe(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterNe, AttrPath: "active", Value: "true"}
	applyUserFilterAST(node, query)
	assert.Equal(t, "disabled", query.Status)
}

func TestApplyUserFilterAST_ActiveFalse(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterEq, AttrPath: "active", Value: "false"}
	applyUserFilterAST(node, query)
	assert.Equal(t, "disabled", query.Status)
}

func TestApplyUserFilterAST_ExternalIdEq(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterEq, AttrPath: "externalId", Value: "ext-1"}
	applyUserFilterAST(node, query)
	assert.Equal(t, "external_id", query.SearchField)
	assert.Equal(t, "ext-1", query.SearchValue)
}

func TestApplyUserFilterAST_DisplayNameCo(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterCo, AttrPath: "displayName", Value: "alice"}
	applyUserFilterAST(node, query)
	assert.Equal(t, "alice", query.Keyword)
}

func TestApplyUserFilterAST_And(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{
		Op:    FilterAnd,
		Left:  &FilterNode{Op: FilterEq, AttrPath: "userName", Value: "alice"},
		Right: &FilterNode{Op: FilterEq, AttrPath: "active", Value: "true"},
	}
	applyUserFilterAST(node, query)
	assert.Equal(t, "username", query.SearchField)
	assert.Equal(t, "alice", query.SearchValue)
	assert.Equal(t, "active", query.Status)
}

func TestApplyUserFilterAST_Or(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{
		Op:   FilterOr,
		Left: &FilterNode{Op: FilterEq, AttrPath: "userName", Value: "alice"},
	}
	applyUserFilterAST(node, query)
	assert.Equal(t, "username", query.SearchField)
}

func TestApplyUserFilterAST_Not(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{
		Op:   FilterNot,
		Left: &FilterNode{Op: FilterEq, AttrPath: "userName", Value: "alice"},
	}
	applyUserFilterAST(node, query)
	// NOT is not directly supported, should be a no-op
	assert.Empty(t, query.SearchField)
}

func TestApplyUserFilterAST_Pr(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterPr, AttrPath: "userName"}
	applyUserFilterAST(node, query)
	// "pr" is not directly supported, should be a no-op
	assert.Empty(t, query.SearchField)
}

func TestApplyUserFilterAST_DefaultFallback(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterEq, AttrPath: "unknownAttr", Value: "test"}
	applyUserFilterAST(node, query)
	assert.Equal(t, "test", query.Keyword)
}

func TestApplyPatchReplace_Active_Bool(t *testing.T) {
	user := &users.User{IsForbidden: true}
	err := applyPatchReplace(user, ScimOperation{Path: "active", Value: true})
	require.NoError(t, err)
	assert.False(t, user.IsForbidden)
}

func TestApplyPatchReplace_Active_StringTrue(t *testing.T) {
	user := &users.User{IsForbidden: true}
	err := applyPatchReplace(user, ScimOperation{Path: "active", Value: "true"})
	require.NoError(t, err)
	assert.False(t, user.IsForbidden)
}

func TestApplyPatchReplace_Active_StringFalse(t *testing.T) {
	user := &users.User{IsForbidden: false}
	err := applyPatchReplace(user, ScimOperation{Path: "active", Value: "false"})
	require.NoError(t, err)
	assert.True(t, user.IsForbidden)
}

func TestApplyPatchReplace_Active_InvalidType(t *testing.T) {
	user := &users.User{}
	err := applyPatchReplace(user, ScimOperation{Path: "active", Value: 123})
	assert.Error(t, err)
}

func TestApplyPatchReplace_Active_InvalidString(t *testing.T) {
	user := &users.User{}
	err := applyPatchReplace(user, ScimOperation{Path: "active", Value: "notabool"})
	assert.Error(t, err)
}

func TestApplyPatchReplace_Username(t *testing.T) {
	user := &users.User{Username: "old"}
	err := applyPatchReplace(user, ScimOperation{Path: "username", Value: "new"})
	require.NoError(t, err)
	assert.Equal(t, "new", user.Username)
}

func TestApplyPatchReplace_DisplayName(t *testing.T) {
	user := &users.User{}
	err := applyPatchReplace(user, ScimOperation{Path: "displayName", Value: "New Name"})
	require.NoError(t, err)
	assert.Equal(t, "New Name", user.Profile.DisplayName)
}

func TestApplyPatchReplace_NameGivenName(t *testing.T) {
	user := &users.User{}
	err := applyPatchReplace(user, ScimOperation{Path: "name.givenName", Value: "Alice"})
	require.NoError(t, err)
	assert.Equal(t, "Alice", user.Profile.FirstName)
}

func TestApplyPatchReplace_NameFamilyName(t *testing.T) {
	user := &users.User{}
	err := applyPatchReplace(user, ScimOperation{Path: "name.familyName", Value: "Smith"})
	require.NoError(t, err)
	assert.Equal(t, "Smith", user.Profile.LastName)
}

func TestApplyPatchReplace_Emails(t *testing.T) {
	user := &users.User{}
	err := applyPatchReplace(user, ScimOperation{Path: "emails", Value: map[string]any{"value": "alice@example.com"}})
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", user.Profile.Email)
}

func TestApplyPatchReplace_PhoneNumbers(t *testing.T) {
	user := &users.User{}
	err := applyPatchReplace(user, ScimOperation{Path: "phoneNumbers", Value: map[string]any{"value": "+1234567890"}})
	require.NoError(t, err)
	assert.Equal(t, "+1234567890", user.Profile.Phone)
}

func TestApplyPatchReplace_NoPath(t *testing.T) {
	user := &users.User{}
	err := applyPatchReplace(user, ScimOperation{Path: "", Value: map[string]any{
		"active":      true,
		"userName":    "alice",
		"displayName": "Alice",
	}})
	require.NoError(t, err)
	assert.False(t, user.IsForbidden)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, "Alice", user.Profile.DisplayName)
}

func TestApplyPatchAdd(t *testing.T) {
	user := &users.User{Username: "old"}
	err := applyPatchAdd(user, ScimOperation{Path: "username", Value: "new"})
	require.NoError(t, err)
	assert.Equal(t, "new", user.Username)
}

func TestApplyPatchRemove_DisplayName(t *testing.T) {
	user := &users.User{Profile: users.UserProfile{DisplayName: "Alice"}}
	applyPatchRemove(user, ScimOperation{Path: "displayName"})
	assert.Equal(t, "", user.Profile.DisplayName)
}

func TestApplyPatchRemove_NameGivenName(t *testing.T) {
	user := &users.User{Profile: users.UserProfile{FirstName: "Alice"}}
	applyPatchRemove(user, ScimOperation{Path: "name.givenName"})
	assert.Equal(t, "", user.Profile.FirstName)
}

func TestApplyPatchRemove_Emails(t *testing.T) {
	user := &users.User{Profile: users.UserProfile{Email: "alice@example.com"}}
	applyPatchRemove(user, ScimOperation{Path: "emails"})
	assert.Equal(t, "", user.Profile.Email)
}

func TestApplyPatchRemove_EmailsValue(t *testing.T) {
	user := &users.User{Profile: users.UserProfile{Email: "alice@example.com"}}
	applyPatchRemove(user, ScimOperation{Path: "emails.value"})
	assert.Equal(t, "", user.Profile.Email)
}

func TestApplyPatchRemove_PhoneNumbers(t *testing.T) {
	user := &users.User{Profile: users.UserProfile{Phone: "+1234567890"}}
	applyPatchRemove(user, ScimOperation{Path: "phoneNumbers"})
	assert.Equal(t, "", user.Profile.Phone)
}

func TestApplyPatchRemove_Username_NoOp(t *testing.T) {
	user := &users.User{Username: "alice"}
	applyPatchRemove(user, ScimOperation{Path: "username"})
	assert.Equal(t, "alice", user.Username) // cannot remove username
}

func TestApplyPatchRemove_Active_NoOp(t *testing.T) {
	user := &users.User{IsForbidden: false}
	applyPatchRemove(user, ScimOperation{Path: "active"})
	assert.False(t, user.IsForbidden) // cannot remove active status
}

func TestApplyPatchRemove_NoPath(t *testing.T) {
	user := &users.User{Profile: users.UserProfile{DisplayName: "Alice", FirstName: "A"}}
	applyPatchRemove(user, ScimOperation{Path: "", Value: map[string]any{
		"displayName":    "",
		"name.givenName": "",
	}})
	assert.Equal(t, "", user.Profile.DisplayName)
	assert.Equal(t, "", user.Profile.FirstName)
}

func TestApplyGroupPatchReplace_DisplayName(t *testing.T) {
	group := &groups.Group{DisplayName: "Old", Name: "Old"}
	applyGroupPatchReplace(group, ScimOperation{Path: "displayName", Value: "New"})
	assert.Equal(t, "New", group.DisplayName)
	assert.Equal(t, "New", group.Name)
}

func TestApplyGroupPatchReplace_Members_SingleMap(t *testing.T) {
	group := &groups.Group{}
	applyGroupPatchReplace(group, ScimOperation{Path: "members", Value: map[string]any{"value": "user-1"}})
	assert.Equal(t, []string{"user-1"}, group.Users)
}

func TestApplyGroupPatchReplace_Members_Array(t *testing.T) {
	group := &groups.Group{}
	applyGroupPatchReplace(group, ScimOperation{Path: "members", Value: []interface{}{
		map[string]any{"value": "user-1"},
		map[string]any{"value": "user-2"},
	}})
	assert.Equal(t, []string{"user-1", "user-2"}, group.Users)
}

func TestApplyGroupPatchReplace_NoPath(t *testing.T) {
	group := &groups.Group{}
	applyGroupPatchReplace(group, ScimOperation{Path: "", Value: map[string]any{"displayName": "Admins"}})
	assert.Equal(t, "Admins", group.DisplayName)
	assert.Equal(t, "Admins", group.Name)
}

func TestApplyGroupPatchAdd_Members_SingleMap(t *testing.T) {
	group := &groups.Group{Users: []string{"user-1"}}
	applyGroupPatchAdd(group, ScimOperation{Path: "members", Value: map[string]any{"value": "user-2"}})
	assert.Contains(t, group.Users, "user-1")
	assert.Contains(t, group.Users, "user-2")
}

func TestApplyGroupPatchAdd_Members_Array(t *testing.T) {
	group := &groups.Group{Users: []string{"user-1"}}
	applyGroupPatchAdd(group, ScimOperation{Path: "members", Value: []interface{}{
		map[string]any{"value": "user-2"},
		map[string]any{"value": "user-3"},
	}})
	assert.Len(t, group.Users, 3)
}

func TestApplyGroupPatchAdd_Default(t *testing.T) {
	group := &groups.Group{DisplayName: "Old"}
	applyGroupPatchAdd(group, ScimOperation{Path: "displayName", Value: "New"})
	assert.Equal(t, "New", group.DisplayName)
}

func TestApplyGroupPatchRemove_Members_SingleMap(t *testing.T) {
	group := &groups.Group{Users: []string{"user-1", "user-2"}}
	applyGroupPatchRemove(group, ScimOperation{Path: "members", Value: map[string]any{"value": "user-1"}})
	assert.Equal(t, []string{"user-2"}, group.Users)
}

func TestApplyGroupPatchRemove_Members_Array(t *testing.T) {
	group := &groups.Group{Users: []string{"user-1", "user-2", "user-3"}}
	applyGroupPatchRemove(group, ScimOperation{Path: "members", Value: []interface{}{
		map[string]any{"value": "user-1"},
		map[string]any{"value": "user-3"},
	}})
	assert.Equal(t, []string{"user-2"}, group.Users)
}

func TestApplyGroupPatchRemove_Members_Default(t *testing.T) {
	group := &groups.Group{Users: []string{"user-1", "user-2"}}
	applyGroupPatchRemove(group, ScimOperation{Path: "members", Value: "some-string"})
	assert.Nil(t, group.Users)
}

func TestApplyGroupPatchRemove_DisplayName(t *testing.T) {
	group := &groups.Group{DisplayName: "Admins", Name: "Admins"}
	applyGroupPatchRemove(group, ScimOperation{Path: "displayName"})
	assert.Equal(t, "", group.DisplayName)
	assert.Equal(t, "", group.Name)
}

func TestRemoveStringFromSlice(t *testing.T) {
	result := removeStringFromSlice([]string{"a", "b", "c"}, "b")
	assert.Equal(t, []string{"a", "c"}, result)
}

func TestRemoveStringFromSlice_NotFound(t *testing.T) {
	result := removeStringFromSlice([]string{"a", "b"}, "c")
	assert.Equal(t, []string{"a", "b"}, result)
}

func TestRemoveStringFromSlice_Empty(t *testing.T) {
	result := removeStringFromSlice([]string{}, "a")
	assert.Empty(t, result)
}

func TestRemoveStringFromSlice_Nil(t *testing.T) {
	result := removeStringFromSlice(nil, "a")
	assert.Empty(t, result)
}

func TestApplyUserComparisonFilter_UsernameEq(t *testing.T) {
	query := &users.ListQuery{}
	node := &FilterNode{Op: FilterEq, AttrPath: "username", Value: "alice"}
	applyUserComparisonFilter(node, query)
	assert.Equal(t, "username", query.SearchField)
	assert.Equal(t, "alice", query.SearchValue)
}

func TestApplyUserComparisonFilter_UsernameKeywordOps(t *testing.T) {
	for _, op := range []FilterOp{FilterSw, FilterCo} {
		query := &users.ListQuery{}
		applyUserComparisonFilter(&FilterNode{Op: op, AttrPath: "username", Value: "ali"}, query)
		assert.Equal(t, "ali", query.Keyword)
	}
}

func TestApplyUserComparisonFilter_UsernameUnsupportedOp(t *testing.T) {
	query := &users.ListQuery{}
	applyUserComparisonFilter(&FilterNode{Op: FilterEw, AttrPath: "username", Value: "ali"}, query)
	assert.Empty(t, query.Keyword)
	assert.Empty(t, query.SearchField)
}
