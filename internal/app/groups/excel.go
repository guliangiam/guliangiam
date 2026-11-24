package groups

import (
	"context"
	"io"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/xuri/excelize/v2"
)

// ImportGroupsFromExcel reads groups from an Excel file and creates them in bulk.
// Expected columns: Name, DisplayName, Type, ParentID
func (s *Service) ImportGroupsFromExcel(ctx context.Context, reader io.Reader, organizationID string) (*BulkImportResult, error) {
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

	var importGroups []ImportGroup
	// Skip header row
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) == 0 || row[0] == "" {
			continue
		}

		group := ImportGroup{
			Name:           safeGetRow(row, 0),
			DisplayName:    safeGetRow(row, 1),
			Type:           safeGetRow(row, 2),
			ParentID:       safeGetRow(row, 3),
			OrganizationID: organizationID,
		}
		importGroups = append(importGroups, group)
	}

	return s.BulkImportGroups(ctx, organizationID, importGroups)
}

func safeGetRow(row []string, idx int) string {
	if idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}
