package sessions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPatchSessionRequest_ToCommand(t *testing.T) {
	state := "revoked"
	req := PatchSessionRequest{
		SessionState: &state,
		Revision:     2,
	}
	cmd := req.toCommand("sess1")
	assert.Equal(t, "sess1", cmd.ID)
	assert.Equal(t, &state, cmd.SessionState)
	assert.Equal(t, int64(2), cmd.Revision)
}

func TestListSessionsRequest_ToQuery(t *testing.T) {
	activeOnly := true
	req := ListSessionsRequest{
		Page:           1,
		PageSize:       20,
		OrganizationID: "org1",
		UserID:         "user1",
		ApplicationID:  "app1",
		State:          "active",
		Keyword:        "test",
		ActiveOnly:     &activeOnly,
	}
	query := req.toQuery()
	assert.Equal(t, 1, query.Page)
	assert.Equal(t, 20, query.PageSize)
	assert.Equal(t, "org1", query.OrganizationID)
	assert.Equal(t, "user1", query.UserID)
	assert.Equal(t, "app1", query.ApplicationID)
	assert.Equal(t, "active", query.State)
	assert.Equal(t, "test", query.Keyword)
	assert.True(t, *query.ActiveOnly)
}
