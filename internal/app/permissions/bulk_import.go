package permissions

import (
	"context"
	"fmt"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BulkImportPermissions imports multiple permissions and returns the result.
func (s *Service) BulkImportPermissions(ctx context.Context, organizationID string, permissions []ImportPermission) (*BulkImportResult, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}

	result := &BulkImportResult{
		Total:  len(permissions),
		Errors: make([]string, 0),
	}

	for i := range permissions {
		perm := &permissions[i]
		// The authenticated tenant scope passed in by the transport is the
		// single authority for the whole batch. A per-row organizationId is
		// request input and must not move a row into another tenant.
		effect := perm.Effect
		if effect == "" {
			effect = "Allow"
		}

		isEnabled := true
		err := s.CreatePermission(ctx, CreatePermissionCommand{
			OrganizationID: organizationID,
			Name:           perm.Name,
			DisplayName:    perm.DisplayName,
			ResourceType:   perm.ResourceType,
			Resources:      perm.Resources,
			Actions:        perm.Actions,
			Users:          perm.Users,
			Groups:         perm.Groups,
			Roles:          perm.Roles,
			Domains:        perm.Domains,
			Effect:         effect,
			IsEnabled:      &isEnabled,
			State:          "Approved",
		})
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("permission %d (%s): %v", i, perm.Name, err))
			continue
		}
		result.Succeeded++
	}

	return result, nil
}
