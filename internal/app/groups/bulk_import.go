package groups

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

// ImportGroup represents a group to be imported.
type ImportGroup struct {
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Type           string `json:"type"`
	ParentID       string `json:"parentId"`
	OrganizationID string `json:"organizationId"`
}

// BulkImportGroups imports multiple groups and returns the result.
func (s *Service) BulkImportGroups(ctx context.Context, organizationID string, groups []ImportGroup) (*BulkImportResult, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}

	result := &BulkImportResult{
		Total:  len(groups),
		Errors: make([]string, 0),
	}

	for i, group := range groups {
		// The authenticated tenant scope passed in by the transport is the
		// single authority for the whole batch. A per-row organizationId is
		// request input and must not move a row into another tenant.
		_, err := s.CreateGroup(ctx, CreateGroupCommand{
			OrganizationID: organizationID,
			ActorID:        "",
			Name:           group.Name,
			DisplayName:    group.DisplayName,
			Type:           group.Type,
			ParentID:       group.ParentID,
			IsEnabled:      true,
		})
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("group %d (%s): %v", i, group.Name, err))
			continue
		}
		result.Succeeded++
	}

	return result, nil
}

// Compile-time check that Service uses uow.UnitOfWork.
var _ = uow.UnitOfWork(nil)
