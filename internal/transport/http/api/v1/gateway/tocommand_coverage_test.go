package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListServersRequest_toQuery_v1(t *testing.T) {
	r := ListServersRequest{
		OrganizationID: "org-1",
	}
	result := r.toQuery()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestCreateServerRequest_toCommand_v1(t *testing.T) {
	r := CreateServerRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestUpdateServerRequest_toCommand_v1(t *testing.T) {
	r := UpdateServerRequest{
		ID: "test-id",
	}
	result := r.toCommand()
	assert.Equal(t, "test-id", result.ID)
	_ = result
}

func TestListSitesRequest_toQuery_v1(t *testing.T) {
	r := ListSitesRequest{
		OrganizationID: "org-1",
	}
	result := r.toQuery()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestCreateSiteRequest_toCommand_v1(t *testing.T) {
	r := CreateSiteRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestUpdateSiteRequest_toCommand_v1(t *testing.T) {
	r := UpdateSiteRequest{
		ID: "test-id",
	}
	result := r.toCommand()
	assert.Equal(t, "test-id", result.ID)
	_ = result
}
