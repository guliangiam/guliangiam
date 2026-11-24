package roles

import (
	"context"
	"fmt"
	"strings"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BulkImportResult contains the result of a bulk import operation.
type BulkImportResult struct {
	Total     int      `json:"total"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
}

// ImportRole represents a role to be imported.
type ImportRole struct {
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Description    string `json:"description"`
	OrganizationID string `json:"organizationId"`
}

// BulkImportRoles imports multiple roles and returns the result.
func (s *Service) BulkImportRoles(ctx context.Context, organizationID string, roles []ImportRole) (*BulkImportResult, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}

	result := &BulkImportResult{
		Total:  len(roles),
		Errors: make([]string, 0),
	}

	for i, role := range roles {
		// The authenticated tenant scope passed in by the transport is the
		// single authority for the whole batch. A per-row organizationId is
		// request input and must not move a row into another tenant.
		err := s.CreateRole(ctx, CreateRoleCommand{
			OrganizationID: organizationID,
			Name:           role.Name,
			DisplayName:    role.DisplayName,
			Description:    role.Description,
		})
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("role %d (%s): %v", i, role.Name, err))
			continue
		}
		result.Succeeded++
	}

	return result, nil
}

// Compile-time check that Service uses uow.UnitOfWork.
var _ = uow.UnitOfWork(nil)
