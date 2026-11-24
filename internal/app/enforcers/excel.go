package enforcers

import (
	"context"
	"fmt"
	"io"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/xuri/excelize/v2"
)

// ImportPoliciesFromExcel reads policies from an Excel file and imports them
// into the specified enforcer, replacing all existing policies.
// Expected columns: PType, V0, V1, V2, V3, V4, V5
// Each row represents one policy rule. Empty cells are skipped.
func (s Service) ImportPoliciesFromExcel(ctx context.Context, organizationID, enforcerID string, reader io.Reader) (*PolicyUploadResult, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	enforcer, err := s.UOW.Enforcers().GetByIDForOrganization(ctx, enforcerID, organizationID)
	if err != nil {
		return nil, err
	}

	if !enforcer.IsEnabled {
		return nil, apperrors.NewInvalidArgument("enforcer is disabled")
	}

	modelText, err := s.getModelText(ctx, enforcer.OrganizationID, enforcer.ModelID)
	if err != nil {
		return nil, err
	}

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
		return &PolicyUploadResult{}, nil
	}

	var policies [][]string
	result := &PolicyUploadResult{
		Errors: make([]string, 0),
	}

	// Skip header row
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) == 0 || row[0] == "" {
			continue
		}

		policy := parsePolicyRow(row)
		if len(policy) == 0 {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: empty policy", i+1))
			continue
		}

		policies = append(policies, policy)
	}

	result.Total = len(policies)

	// 原子化替换（BE-D-8）：中途失败恢复快照，不再留下全空策略。
	if err := s.replacePolicies(enforcerID, modelText, policies); err != nil {
		return nil, err
	}
	result.Succeeded = len(policies)
	s.invalidatePermissionCache()

	if err := s.auditPolicyWrite(ctx, enforcer.OrganizationID, enforcerID, "enforcers.policy.uploaded", map[string]any{
		"total":     result.Total,
		"succeeded": result.Succeeded,
		"failed":    result.Failed,
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// parsePolicyRow converts an Excel row to a policy string slice,
// trimming trailing empty values.
func parsePolicyRow(row []string) []string {
	var policy []string
	for _, cell := range row {
		policy = append(policy, strings.TrimSpace(cell))
	}

	// Trim trailing empty values
	for len(policy) > 0 && policy[len(policy)-1] == "" {
		policy = policy[:len(policy)-1]
	}

	return policy
}
