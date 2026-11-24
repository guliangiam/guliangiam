package users

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BulkImportResult contains the result of a bulk import operation.
type BulkImportResult struct {
	Total     int      `json:"total"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
}

// ImportUser represents a user to be imported.
type ImportUser struct {
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Password       string `json:"password"`
	OrganizationID string `json:"organizationId"`
}

// BulkImportUsers imports multiple users and returns the result.
func (s *Service) BulkImportUsers(ctx context.Context, organizationID string, users []ImportUser) (*BulkImportResult, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	// BE-A-09: JSON 导入与 CSV 导入共用同一行数上限，防止超大请求
	// 触发长时间循环建号（每行含 argon2 哈希）。
	if len(users) > maxCSVImportRows {
		return nil, apperrors.NewUnprocessableEntityf("import exceeds maximum of %d rows", maxCSVImportRows)
	}

	result := &BulkImportResult{
		Total:  len(users),
		Errors: make([]string, 0),
	}

	for i, user := range users {
		// The authenticated tenant scope passed in by the transport is the
		// single authority for the whole import. A per-row organizationId is
		// request input and must not move a row into another tenant.
		_, err := s.CreateUser(ctx, CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: organizationID,
				Username:       user.Username,
				UserType:       "normal",
				Status:         string(userdomain.UsersStateActive_State),
			},
			Profile: CreateUserProfileCommand{
				DisplayName: user.DisplayName,
				Email:       user.Email,
				Phone:       user.Phone,
			},
			InitialPassword: user.Password,
		})
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("user %d (%s): %v", i, user.Username, err))
			continue
		}
		result.Succeeded++
	}

	return result, nil
}

// Compile-time check that Service uses uow.UnitOfWork.
var _ = uow.UnitOfWork(nil)

const maxCSVImportRows = 1000

// CSVImportResult contains the result of a CSV import operation.
type CSVImportResult struct {
	Total   int              `json:"total"`
	Success int              `json:"success"`
	Failed  int              `json:"failed"`
	Errors  []CSVImportError `json:"errors,omitempty"`
}

// CSVImportError represents a single row import failure.
type CSVImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ImportUsersFromCSV imports users from a CSV reader using the provided field mapping.
// fieldMapping maps CSV column names (lowercase) to user field names (e.g. {"姓名": "displayName", "邮箱": "email"}).
func (s *Service) ImportUsersFromCSV(ctx context.Context, organizationID string, reader io.Reader, fieldMapping map[string]string) (*CSVImportResult, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1 // Allow variable number of fields

	// Read header row.
	headers, err := csvReader.Read()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "read CSV header")
	}

	// Normalize headers to lowercase.
	normalizedHeaders := make([]string, len(headers))
	for i, h := range headers {
		normalizedHeaders[i] = strings.TrimSpace(strings.ToLower(h))
	}

	result := &CSVImportResult{}

	for {
		if result.Total >= maxCSVImportRows {
			result.Errors = append(result.Errors, CSVImportError{
				Row:     result.Total + 1,
				Message: fmt.Sprintf("maximum import limit of %d rows reached", maxCSVImportRows),
			})
			break
		}

		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, CSVImportError{
				Row:     result.Total + 2, // +1 for header, +1 for 0-indexed
				Message: "failed to read CSV row: " + err.Error(),
			})
			result.Failed++
			result.Total++
			continue
		}

		result.Total++

		// Build row map from CSV columns.
		rowMap := make(map[string]string)
		for i, val := range record {
			if i < len(normalizedHeaders) {
				rowMap[normalizedHeaders[i]] = strings.TrimSpace(val)
			}
		}

		// Resolve field values using mapping.
		username := resolveCSVField(rowMap, fieldMapping, "username")
		displayName := resolveCSVField(rowMap, fieldMapping, "displayName")
		email := resolveCSVField(rowMap, fieldMapping, fieldEmail)
		phone := resolveCSVField(rowMap, fieldMapping, "phone")
		password := resolveCSVField(rowMap, fieldMapping, "password")

		if username == "" {
			result.Errors = append(result.Errors, CSVImportError{
				Row:     result.Total + 1,
				Message: "username is required",
			})
			result.Failed++
			continue
		}

		_, err = s.CreateUser(ctx, CreateUserWithProfileCommand{
			CreateUserCommand: CreateUserCommand{
				OrganizationID: organizationID,
				Username:       username,
				UserType:       "normal",
				Status:         string(userdomain.UsersStateActive_State),
				RegisterSource: "csv-import",
			},
			Profile: CreateUserProfileCommand{
				DisplayName: displayName,
				Email:       email,
				Phone:       phone,
			},
			InitialPassword: password,
		})
		if err != nil {
			result.Errors = append(result.Errors, CSVImportError{
				Row:     result.Total + 1,
				Message: fmt.Sprintf("create user %s: %s", username, err.Error()),
			})
			result.Failed++
			continue
		}

		result.Success++
	}

	return result, nil
}

// resolveCSVField resolves a target user field from the CSV row using field mapping.
func resolveCSVField(rowMap, fieldMapping map[string]string, targetField string) string {
	// Check mapping: if any CSV column maps to this target field, use it.
	for csvCol, target := range fieldMapping {
		if strings.EqualFold(target, targetField) {
			if val, ok := rowMap[strings.ToLower(csvCol)]; ok {
				return val
			}
		}
	}
	// Fallback: try the target field name directly in the row.
	return rowMap[strings.ToLower(targetField)]
}
