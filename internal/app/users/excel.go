package users

import (
	"context"
	"fmt"
	"io"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/xuri/excelize/v2"
)

// exportPageSize is the batch size used while walking the user list. It is
// deliberately small and constant: the export must return every user, and a
// single `PageSize: 10000` call silently truncated any organization larger than
// that (the count was discarded with `_`), so operators could export a partial
// roster without any error being reported.
const exportPageSize = 500

// ExportUsersToExcel writes all users (filtered by organizationID) to an Excel workbook.
func (s *Service) ExportUsersToExcel(ctx context.Context, organizationID string) (*excelize.File, error) {
	domainUsers := make([]*userdomain.User, 0, exportPageSize)
	for page := 1; ; page++ {
		batch, total, err := s.UOW.Users().List(ctx, userdomain.ListQuery{
			OrganizationID: organizationID,
			Page:           page,
			PageSize:       exportPageSize,
			// 导出列全部来自主行；lean 投影跳过无关聚合加载。
			Lean: true,
		})
		if err != nil {
			return nil, apperrors.WrapInternal(err, "query users")
		}
		domainUsers = append(domainUsers, batch...)

		if len(batch) == 0 || int64(len(domainUsers)) >= total {
			break
		}
	}

	f := excelize.NewFile()
	sheet := "Users"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "create sheet")
	}
	f.SetActiveSheet(idx)

	headers := []string{"ID", "OrganizationID", "Username", "Type", "Status", "IsAdmin", "CreatedTime"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	for row, user := range domainUsers {
		rowNum := row + 2
		values := []interface{}{
			user.ID,
			user.OrganizationID,
			user.Username,
			user.UserType,
			user.Status,
			user.IsAdmin,
			user.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		for col, v := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}

	// Auto-fit column widths
	for i := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, 20)
	}

	return f, nil
}

// ImportUsersResult holds the outcome of an Excel import.
type ImportUsersResult struct {
	SuccessCount int
	FailedRows   []int
	Errors       []string
}

// PreviewUser represents a single row parsed from an Excel file for preview.
type PreviewUser struct {
	Row         int    `json:"row"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	UserType    string `json:"userType"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

// PreviewExcelResult holds the parsed preview data from an Excel file.
type PreviewExcelResult struct {
	TotalRows int           `json:"totalRows"`
	Users     []PreviewUser `json:"users"`
}

// PreviewUsersFromExcel parses an Excel file and returns preview data without importing.
func (s *Service) PreviewUsersFromExcel(ctx context.Context, reader io.Reader, organizationID string) (*PreviewExcelResult, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "open excel")
	}
	defer f.Close()

	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "read rows")
	}

	result := &PreviewExcelResult{}
	if len(rows) < 2 {
		return result, nil
	}

	result.TotalRows = len(rows) - 1
	for i, row := range rows[1:] {
		rowNum := i + 2
		pu := PreviewUser{Row: rowNum}

		if len(row) < 3 {
			pu.Error = "insufficient columns"
			result.Users = append(result.Users, pu)
			continue
		}

		pu.Username = row[2]
		pu.DisplayName = safeGet(row, 3)
		pu.Email = safeGet(row, 4)
		pu.Phone = safeGet(row, 5)
		pu.UserType = safeGet(row, 6)
		pu.Status = safeGet(row, 7)

		// Basic validation
		if pu.Username == "" {
			pu.Error = "username is required"
		}

		result.Users = append(result.Users, pu)
	}

	return result, nil
}

// ImportUsersFromExcel reads users from an Excel file and creates them in bulk.
func (s *Service) ImportUsersFromExcel(ctx context.Context, reader io.Reader, organizationID string) (*ImportUsersResult, error) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "open excel")
	}
	defer f.Close()

	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "read rows")
	}

	result := &ImportUsersResult{}
	if len(rows) < 2 {
		return result, nil
	}

	for i, row := range rows[1:] { // Skip header row
		rowNum := i + 2
		if len(row) < 3 {
			result.FailedRows = append(result.FailedRows, rowNum)
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: insufficient columns", rowNum))
			continue
		}

		cmd := CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: organizationID,
				Username:       row[2],
				UserType:       safeGet(row, 6),
				Status:         safeGet(row, 7),
			},
			Profile: CreateUserProfileCommand{
				DisplayName: safeGet(row, 3),
				Email:       safeGet(row, 4),
				Phone:       safeGet(row, 5),
			},
		}

		if _, err := s.CreateUser(ctx, cmd); err != nil {
			result.FailedRows = append(result.FailedRows, rowNum)
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %s", rowNum, err.Error()))
			continue
		}
		result.SuccessCount++
	}

	return result, nil
}

func safeGet(row []string, idx int) string {
	if idx < len(row) {
		return row[idx]
	}
	return ""
}
