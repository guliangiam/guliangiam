package certs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateCertRequest_toCommand_v1(t *testing.T) {
	r := CreateCertRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchCertRequest_toCommand_v1(t *testing.T) {
	r := PatchCertRequest{}
	result := r.toCommand("test-id")
	_ = result
}
