package products

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateProductRequest_toCommand_v1(t *testing.T) {
	r := CreateProductRequest{
		OrganizationID: "org-1",
	}
	result := r.toCommand()
	assert.Equal(t, "org-1", result.OrganizationID)
	_ = result
}

func TestPatchProductRequest_toCommand_v1(t *testing.T) {
	r := PatchProductRequest{}
	result := r.toCommand("test-id")
	_ = result
}
