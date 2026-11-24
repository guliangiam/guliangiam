package roles

import (
	"context"
	"io"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/xuri/excelize/v2"
)

// ImportRolesFromExcel reads roles from an Excel file and creates them in bulk.
// Expected columns: Name, DisplayName, Description
func (s *Service) ImportRolesFromExcel(ctx context.Context, reader io.Reader, organizationID string) (*BulkImportResult, error) {
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

	var importRoles []ImportRole
	// Skip header row
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) == 0 || row[0] == "" {
			continue
		}

		role := ImportRole{
			Name:           safeGetRow(row, 0),
			DisplayName:    safeGetRow(row, 1),
			Description:    safeGetRow(row, 2),
			OrganizationID: organizationID,
		}
		importRoles = append(importRoles, role)
	}

	return s.BulkImportRoles(ctx, organizationID, importRoles)
}

func safeGetRow(row []string, idx int) string {
	if idx < len(row) {
		return row[idx]
	}
	return ""
}
