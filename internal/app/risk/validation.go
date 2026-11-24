package risk

import (
	"strconv"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/risk"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// The rules in this file keep 手册 4.14.1's 400/422 split:
//   - a missing field is the caller omitting something → 400 invalid_argument;
//   - a value that is present but violates a rule (format, range, closed set,
//     cross-field contradiction) → 422 unprocessable_entity, with `meta` naming
//     the field so the console can point at the input.
//
// Metadata keys follow AC-FEGOV-008: `{field, value}` plus `min`/`max` for
// ranges, `format` for syntax, `expected` for a cross-field rule, and —
// for closed sets — `{field, value, allowed, set}` produced by enumset itself.

const (
	maxReasonLength     = 2000
	maxSignalTypeLen    = 128
	maxPageSize         = platformpagination.MaxPageSize
	minPage             = 1
	minExpectedRevision = 0
)

func validateUUID(field, value string) error {
	if value == "" {
		return apperrors.NewInvalidArgument(field + " is required")
	}
	if !platformuuid.IsValid(value) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			field+" must be a UUID",
			map[string]any{"field": field, "value": value, "format": "uuid"},
		)
	}
	return nil
}

func validateListQuery(orgID string, page, pageSize int) error {
	if err := validateUUID("organizationId", orgID); err != nil {
		return err
	}
	if page < minPage {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"page must be an integer greater than or equal to 1",
			map[string]any{"field": "page", "value": page, "min": minPage},
		)
	}
	if pageSize < minPage || pageSize > maxPageSize {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"pageSize must be an integer between 1 and "+strconv.Itoa(maxPageSize),
			map[string]any{
				"field": "pageSize",
				"value": pageSize,
				"min":   minPage,
				"max":   maxPageSize,
			},
		)
	}
	return nil
}

func validateReason(reason string) error {
	if reason == "" {
		return apperrors.NewInvalidArgument("reason is required")
	}
	if len(reason) > maxReasonLength {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"reason must not exceed 2000 characters",
			map[string]any{"field": "reason", "value": len(reason), "max": maxReasonLength},
		)
	}
	return nil
}

func validateExpectedRevision(expected *int64) error {
	if expected == nil {
		return apperrors.NewInvalidArgument("revision is required")
	}
	if *expected < minExpectedRevision {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"revision must be >= 0",
			map[string]any{"field": "revision", "value": *expected, "min": minExpectedRevision},
		)
	}
	return nil
}

func validateSignalType(signalType string) error {
	if len(signalType) > maxSignalTypeLen {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"signalType must not exceed 128 characters",
			map[string]any{"field": "signalType", "value": len(signalType), "max": maxSignalTypeLen},
		)
	}
	return nil
}

// The three enum rules delegate to the domain sets, so the accepted values and
// the 422 `allowed` list can never drift apart (手册 4.14.3).
func validateDecisionFilter(value *domain.DecisionAction) error {
	if value == nil {
		return nil
	}
	if strings.TrimSpace(string(*value)) == "" {
		return apperrors.NewInvalidArgument("decision must not be blank")
	}
	return domain.DecisionActions.Validate("decision", string(*value))
}

func validateNewDecision(value domain.DecisionAction) error {
	return domain.DecisionActions.ValidateRequired("decision", string(value))
}

func validateGovernanceFilter(value *domain.GovernanceAction) error {
	if value == nil {
		return nil
	}
	if strings.TrimSpace(string(*value)) == "" {
		return apperrors.NewInvalidArgument("action must not be blank")
	}
	return domain.GovernanceActions.Validate("action", string(*value))
}

func validateNewAction(value domain.GovernanceAction) error {
	return domain.GovernanceActions.ValidateRequired("action", string(value))
}

func validateAppealResolution(value domain.AppealStatus) error {
	return domain.AppealResolutions.ValidateRequired("resolution", string(value))
}

func validateChainMatch(chainID, organizationID string) error {
	if chainID != organizationID {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"chainId must match organizationId",
			map[string]any{"field": "chainId", "value": chainID, "expected": organizationID},
		)
	}
	return nil
}

func validateEvaluateRiskCommand(cmd domain.EvaluateRiskCommand) error {
	if err := validateUUID("organizationId", strings.TrimSpace(cmd.OrganizationID)); err != nil {
		return err
	}
	if strings.TrimSpace(string(cmd.LoginChannel)) == "" {
		return apperrors.NewInvalidArgument("loginChannel is required")
	}
	return nil
}
