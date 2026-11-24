package permissions

import (
	"context"
	"io"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/xuri/excelize/v2"
)

// ImportPermissionsFromExcel reads permissions from an Excel file and creates them in bulk.
// Expected columns: Name, DisplayName, ResourceType, Effect, Resources, Actions
// Resources and Actions are comma-separated values within a single cell.
func (s *Service) ImportPermissionsFromExcel(ctx context.Context, reader io.Reader, organizationID string) (*BulkImportResult, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "open excel")
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "read rows")
	}

	if len(rows) == 0 {
		return &BulkImportResult{}, nil
	}

	var importPerms []ImportPermission
	// Skip header row
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) == 0 || row[0] == "" {
			continue
		}

		perm := ImportPermission{
			Name:           safeGetPermRow(row, 0),
			DisplayName:    safeGetPermRow(row, 1),
			ResourceType:   safeGetPermRow(row, 2),
			Effect:         safeGetPermRow(row, 3),
			Resources:      splitCSV(safeGetPermRow(row, 4)),
			Actions:        splitCSV(safeGetPermRow(row, 5)),
			OrganizationID: organizationID,
		}
		if perm.Effect == "" {
			perm.Effect = "Allow"
		}
		importPerms = append(importPerms, perm)
	}

	return s.BulkImportPermissions(ctx, organizationID, importPerms)
}

func safeGetPermRow(row []string, idx int) string {
	if idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
