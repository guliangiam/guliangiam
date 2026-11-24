package ldapsync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateSyncConfigRequest_ToCommand(t *testing.T) {
	req := &CreateSyncConfigRequest{
		OrganizationID:      "org1",
		LdapID:              "ldap1",
		SyncIntervalMinutes: 30,
		BaseDN:              "dc=example,dc=com",
		UserFilter:          "(objectClass=user)",
		GroupFilter:         "(objectClass=group)",
		Enabled:             true,
	}
	result := req.toCommand()
	assert.Equal(t, "org1", result.OrganizationID)
	assert.Equal(t, "ldap1", result.LdapID)
	assert.Equal(t, 30, result.SyncIntervalMinutes)
	assert.Equal(t, "dc=example,dc=com", result.BaseDN)
	assert.Equal(t, "(objectClass=user)", result.UserFilter)
	assert.Equal(t, "(objectClass=group)", result.GroupFilter)
	assert.True(t, result.Enabled)
}

func TestPatchSyncConfigRequest_ToPatch_AllFields(t *testing.T) {
	minutes := 15
	baseDN := "dc=new,dc=com"
	userFilter := "(objectClass=newUser)"
	groupFilter := "(objectClass=newGroup)"
	enabled := false

	req := &PatchSyncConfigRequest{
		SyncIntervalMinutes: &minutes,
		BaseDN:              &baseDN,
		UserFilter:          &userFilter,
		GroupFilter:         &groupFilter,
		Enabled:             &enabled,
	}
	patch := req.toPatch()

	assert.NotNil(t, patch.SyncIntervalMinutes)
	assert.Equal(t, 15, *patch.SyncIntervalMinutes)
	assert.NotNil(t, patch.BaseDN)
	assert.Equal(t, "dc=new,dc=com", *patch.BaseDN)
	assert.NotNil(t, patch.UserFilter)
	assert.Equal(t, "(objectClass=newUser)", *patch.UserFilter)
	assert.NotNil(t, patch.GroupFilter)
	assert.Equal(t, "(objectClass=newGroup)", *patch.GroupFilter)
	assert.NotNil(t, patch.Enabled)
	assert.False(t, *patch.Enabled)
}

func TestPatchSyncConfigRequest_ToPatch_PartialFields(t *testing.T) {
	minutes := 15
	req := &PatchSyncConfigRequest{
		SyncIntervalMinutes: &minutes,
		// Other fields are nil
	}
	patch := req.toPatch()

	assert.NotNil(t, patch.SyncIntervalMinutes)
	assert.Equal(t, 15, *patch.SyncIntervalMinutes)
	assert.Nil(t, patch.BaseDN)      // unchanged -> not applied by the service
	assert.Nil(t, patch.UserFilter)  // unchanged
	assert.Nil(t, patch.GroupFilter) // unchanged
	assert.Nil(t, patch.Enabled)     // unchanged
}

func TestPatchSyncConfigRequest_ToPatch_NoneSet(t *testing.T) {
	req := &PatchSyncConfigRequest{}
	patch := req.toPatch()

	assert.Nil(t, patch.SyncIntervalMinutes)
	assert.Nil(t, patch.BaseDN)
	assert.Nil(t, patch.UserFilter)
	assert.Nil(t, patch.GroupFilter)
	assert.Nil(t, patch.Enabled)
}
