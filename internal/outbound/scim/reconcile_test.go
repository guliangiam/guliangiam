package scim

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileUsers(t *testing.T) {
	expected := []UserRecord{
		{ExternalID: "e-1", UserName: "alice", Active: true, Attributes: map[string]string{"email": "alice@example.com", "title": "Engineer"}},
		{ExternalID: "e-2", UserName: "bob", Active: false, Attributes: map[string]string{"email": "bob@example.com"}},
		{ExternalID: "e-3", UserName: "carol", Active: true},
	}
	remote := []UserRecord{
		{ExternalID: "e-1", UserName: "alice", Active: true, Attributes: map[string]string{"email": "alice@old.example.com", "title": "Engineer", "extra": "ignored"}},
		{ExternalID: "e-2", UserName: "bob", Active: true, Attributes: map[string]string{"email": "bob@example.com"}},
		{ExternalID: "e-4", UserName: "dave", Active: true},
	}

	report := ReconcileUsers(expected, remote)
	require.True(t, report.HasDrift())
	assert.Equal(t, []string{"e-3"}, report.Missing)
	assert.Equal(t, []string{"e-4"}, report.Extra)
	require.Len(t, report.AttributeMismatch, 1)
	assert.Equal(t, AttributeDrift{UserName: "e-1", Field: "email", Expected: "alice@example.com", Remote: "alice@old.example.com"}, report.AttributeMismatch[0])
	require.Len(t, report.StateMismatch, 1)
	assert.Equal(t, StateDrift{UserName: "e-2", Expected: false, Remote: true}, report.StateMismatch[0])
}

func TestReconcileUsers_NoDrift(t *testing.T) {
	users := []UserRecord{{ExternalID: "e-1", UserName: "alice", Active: true}}
	report := ReconcileUsers(users, users)
	assert.False(t, report.HasDrift())

	encoded, err := json.Marshal(report)
	require.NoError(t, err)
	assert.JSONEq(t, `{"missing":null,"extra":null,"attributeMismatch":null,"stateMismatch":null}`, string(encoded))
}

func TestReconcileUsers_FallsBackToUserNameAndIgnoresCase(t *testing.T) {
	expected := []UserRecord{{UserName: "Alice", Active: true}}
	remote := []UserRecord{{UserName: "alice", Active: true}}
	report := ReconcileUsers(expected, remote)
	assert.False(t, report.HasDrift())
}

func TestReconcileGroups(t *testing.T) {
	expected := []GroupRecord{
		{ExternalID: "g-1", DisplayName: "engineering", Members: []string{"u-1", "u-2"}},
		{ExternalID: "g-2", DisplayName: "ops", Members: []string{"u-3"}},
		{ExternalID: "g-3", DisplayName: "finance", Members: []string{"u-4"}},
	}
	remote := []GroupRecord{
		{ExternalID: "g-1", DisplayName: "engineering", Members: []string{"u-1", "u-9"}},
		{ExternalID: "g-2", DisplayName: "ops", Members: []string{"u-3"}},
		{ExternalID: "g-4", DisplayName: "sales", Members: []string{"u-5"}},
	}

	report := ReconcileGroups(expected, remote)
	require.True(t, report.HasDrift())
	assert.Equal(t, []string{"g-3"}, report.Missing)
	assert.Equal(t, []string{"g-4"}, report.Extra)
	require.Len(t, report.MemberMismatch, 1)
	assert.Equal(t, GroupMemberDrift{Group: "g-1", MissingMembers: []string{"u-2"}, ExtraMembers: []string{"u-9"}}, report.MemberMismatch[0])
}

func TestReconcileGroups_NoDrift(t *testing.T) {
	groups := []GroupRecord{{ExternalID: "g-1", DisplayName: "engineering", Members: []string{"u-1", "u-2"}}}
	report := ReconcileGroups(groups, groups)
	assert.False(t, report.HasDrift())
}
