package tokens

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListTokensRequest_toQuery_v1(t *testing.T) {
	r := ListTokensRequest{
		OrganizationID: "org-1",
	}
	result := r.toQuery()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestCreateTokenRequest_toCommand_v1(t *testing.T) {
	r := CreateTokenRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestRevokeTokensRequest_toCommand_v1(t *testing.T) {
	r := RevokeTokensRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestIntrospectTokenRequest_toCommand_v1(t *testing.T) {
	r := IntrospectTokenRequest{}
	result := r.toCommand()
	_ = result
}

func TestRefreshTokenRequest_toCommand_v1(t *testing.T) {
	r := RefreshTokenRequest{}
	result := r.toCommand()
	_ = result
}
