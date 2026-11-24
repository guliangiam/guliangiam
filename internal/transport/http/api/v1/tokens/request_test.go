package tokens

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListTokensRequest_ToQuery(t *testing.T) {
	activeOnly := true
	req := ListTokensRequest{
		Page:           1,
		PageSize:       10,
		OrganizationID: "org1",
		UserID:         "user1",
		ApplicationID:  "app1",
		ActiveOnly:     &activeOnly,
		Keyword:        "test",
	}
	query := req.toQuery()
	assert.Equal(t, 1, query.Page)
	assert.Equal(t, 10, query.PageSize)
	assert.Equal(t, "org1", query.OrganizationID)
	assert.Equal(t, "user1", query.UserID)
}

func TestCreateTokenRequest_ToCommand(t *testing.T) {
	req := CreateTokenRequest{
		OrganizationID:          "org1",
		UserID:                  "user1",
		ApplicationID:           "app1",
		Scopes:                  []string{"read", "write"},
		Audience:                []string{"api1"},
		Metadata:                map[string]any{"key": "value"},
		ExpiresInSeconds:        3600,
		IssueRefreshToken:       true,
		RefreshExpiresInSeconds: 86400,
	}
	cmd := req.toCommand()
	assert.Equal(t, "org1", cmd.OrganizationID)
	assert.Equal(t, "user1", cmd.UserID)
	assert.Equal(t, "app1", cmd.ApplicationID)
	assert.Equal(t, []string{"read", "write"}, cmd.Scopes)
	assert.Equal(t, int64(3600), cmd.ExpiresInSeconds)
	assert.True(t, cmd.IssueRefreshToken)
}

func TestRevokeTokensRequest_ToCommand(t *testing.T) {
	req := RevokeTokensRequest{
		OrganizationID: "org1",
		UserID:         "user1",
		ApplicationID:  "app1",
	}
	cmd := req.toCommand()
	assert.Equal(t, "org1", cmd.OrganizationID)
	assert.Equal(t, "user1", cmd.UserID)
}

func TestIntrospectTokenRequest_ToCommand(t *testing.T) {
	req := IntrospectTokenRequest{
		Token:         "token123",
		TokenTypeHint: "access_token",
	}
	cmd := req.toCommand()
	assert.Equal(t, "token123", cmd.Token)
	assert.Equal(t, "access_token", cmd.TokenTypeHint)
}

func TestRefreshTokenRequest_ToCommand(t *testing.T) {
	req := RefreshTokenRequest{RefreshToken: "refresh123"}
	cmd := req.toCommand()
	assert.Equal(t, "refresh123", cmd.RefreshToken)
}
