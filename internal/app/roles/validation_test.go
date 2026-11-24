package roles

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func assertRoleAppError(t *testing.T, err error, code apperrors.Code) *apperrors.Error {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T (%v)", err, err)
	}
	if appErr.Code != code {
		t.Fatalf("code = %q, want %q", appErr.Code, code)
	}
	return appErr
}

func TestValidateCreateRoleCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateRoleCommand{
			OrganizationID: "org-001",
			Name:           "admin",
			DisplayName:    "Admin",
		}
		assert.NoError(t, validateCreateRoleCommand(cmd))
	})

	t.Run("missing_organization_id", func(t *testing.T) {
		cmd := CreateRoleCommand{Name: "admin", DisplayName: "Admin"}
		assertRoleAppError(t, validateCreateRoleCommand(cmd), apperrors.CodeInvalidArgument)
	})

	t.Run("missing_name", func(t *testing.T) {
		cmd := CreateRoleCommand{OrganizationID: "org-001", DisplayName: "Admin"}
		assertRoleAppError(t, validateCreateRoleCommand(cmd), apperrors.CodeInvalidArgument)
	})

	t.Run("missing_display_name", func(t *testing.T) {
		cmd := CreateRoleCommand{OrganizationID: "org-001", Name: "admin"}
		assertRoleAppError(t, validateCreateRoleCommand(cmd), apperrors.CodeInvalidArgument)
	})

	t.Run("duplicate_group_ids", func(t *testing.T) {
		cmd := CreateRoleCommand{
			OrganizationID: "org-001",
			Name:           "admin",
			DisplayName:    "Admin",
			GroupIDs:       []string{"g1", "g1"},
		}
		assertRoleAppError(t, validateCreateRoleCommand(cmd), apperrors.CodeUnprocessableEntity)
	})

	t.Run("blank_group_id", func(t *testing.T) {
		cmd := CreateRoleCommand{
			OrganizationID: "org-001",
			Name:           "admin",
			DisplayName:    "Admin",
			GroupIDs:       []string{""},
		}
		assertRoleAppError(t, validateCreateRoleCommand(cmd), apperrors.CodeUnprocessableEntity)
	})
}

func TestValidatePatchRoleCommand(t *testing.T) {
	t.Run("valid_all_nil", func(t *testing.T) {
		cmd := PatchRoleCommand{}
		assert.NoError(t, validatePatchRoleCommand(cmd))
	})

	t.Run("blank_display_name", func(t *testing.T) {
		blank := ""
		cmd := PatchRoleCommand{DisplayName: &blank}
		assertRoleAppError(t, validatePatchRoleCommand(cmd), apperrors.CodeInvalidArgument)
	})

	t.Run("duplicate_group_ids", func(t *testing.T) {
		cmd := PatchRoleCommand{GroupIDs: []string{"g1", "g1"}}
		assertRoleAppError(t, validatePatchRoleCommand(cmd), apperrors.CodeUnprocessableEntity)
	})
}

func TestValidateUniqueStrings(t *testing.T) {
	t.Run("unique_values", func(t *testing.T) {
		assert.NoError(t, validateUniqueStrings([]string{"a", "b", "c"}, "field", true))
	})

	t.Run("duplicate_values", func(t *testing.T) {
		assertRoleAppError(t, validateUniqueStrings([]string{"a", "b", "a"}, "field", true), apperrors.CodeUnprocessableEntity)
	})

	t.Run("blank_value_rejected", func(t *testing.T) {
		assertRoleAppError(t, validateUniqueStrings([]string{""}, "field", true), apperrors.CodeUnprocessableEntity)
	})

	t.Run("blank_value_allowed", func(t *testing.T) {
		assert.NoError(t, validateUniqueStrings([]string{""}, "field", false))
	})

	t.Run("empty_slice", func(t *testing.T) {
		assert.NoError(t, validateUniqueStrings([]string{}, "field", true))
	})
}
