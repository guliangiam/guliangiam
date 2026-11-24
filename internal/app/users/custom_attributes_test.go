package users

import (
	"testing"

	"github.com/stretchr/testify/require"

	customattributes "github.com/guliangiam/guliangiam/internal/domain/customattributes"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestValidateProfilePropertiesAgainstDefinitions(t *testing.T) {
	definitions := []customattributes.Definition{{
		OrganizationID: "org-1", Key: "department", DataType: customattributes.DataTypeString,
		Writable: true, Visibility: customattributes.VisibilityPrivate, Enabled: true,
	}}
	require.NoError(t, ValidateProfilePropertiesAgainstDefinitions(nil, definitions))
	require.NoError(t, ValidateProfilePropertiesAgainstDefinitions(map[string]any{"department": "platform"}, definitions))

	err := ValidateProfilePropertiesAgainstDefinitions(map[string]any{"unknown": "x"}, definitions)
	require.Error(t, err)
	require.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
}
