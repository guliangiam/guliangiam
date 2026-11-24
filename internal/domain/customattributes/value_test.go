package customattributes

import (
	"testing"

	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func definition(dataType DataType, options ...string) Definition {
	return Definition{
		OrganizationID: "org-1", Key: "department", DataType: dataType,
		Options: options, Writable: true, Visibility: VisibilityPrivate, Enabled: true,
	}
}

func TestDefinitionValidate(t *testing.T) {
	require.NoError(t, definition(DataTypeString).Validate())
	require.NoError(t, definition(DataTypeEnum, "eng", "ops").Validate())

	badKey := definition(DataTypeString)
	badKey.Key = "Department"
	require.Error(t, badKey.Validate())

	missingOptions := definition(DataTypeEnum)
	require.Error(t, missingOptions.Validate())

	nonEnumOptions := definition(DataTypeString, "x")
	require.Error(t, nonEnumOptions.Validate())

	badVisibility := definition(DataTypeString)
	badVisibility.Visibility = "everyone"
	require.Error(t, badVisibility.Validate())
}

func TestValidateProperties(t *testing.T) {
	defs := []Definition{
		definition(DataTypeString),
		{OrganizationID: "org-1", Key: "age", DataType: DataTypeNumber, Writable: true, Visibility: VisibilityPrivate, Enabled: true},
		{OrganizationID: "org-1", Key: "tier", DataType: DataTypeEnum, Options: []string{"gold", "silver"}, Writable: true, Visibility: VisibilityPrivate, Enabled: true},
		{OrganizationID: "org-1", Key: "joined", DataType: DataTypeDate, Writable: true, Visibility: VisibilityPrivate, Enabled: true, Required: true},
	}
	require.NoError(t, ValidateProperties(map[string]any{
		"department": "platform", "age": float64(30), "tier": "gold", "joined": "2026-01-02",
	}, defs))

	cases := map[string]map[string]any{
		"unregistered": {"unknown": "x", "joined": "2026-01-02"},
		"wrong type":   {"age": "thirty", "joined": "2026-01-02"},
		"enum":         {"tier": "bronze", "joined": "2026-01-02"},
		"date":         {"joined": "not-a-date"},
		"required":     {"department": "platform"},
	}
	for name, properties := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateProperties(properties, defs)
			require.Error(t, err)
			require.Equal(t, apperrors.CodeUnprocessableEntity, apperrors.FromError(err).Code)
		})
	}
}

func TestValidateProperties_ReadOnlyAndDisabled(t *testing.T) {
	readOnly := definition(DataTypeString)
	readOnly.Writable = false
	err := ValidateProperties(map[string]any{"department": "x"}, []Definition{readOnly})
	require.Error(t, err)

	disabled := definition(DataTypeString)
	disabled.Enabled = false
	err = ValidateProperties(map[string]any{"department": "x"}, []Definition{disabled})
	require.Error(t, err)
}
