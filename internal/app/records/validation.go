package records

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// The records table declares resource_id / user_agent as VARCHAR(255); the
// metadata column is JSONB with no declared ceiling. 32 KiB is the existing
// single-record structured-payload ceiling (手册 12.10) that this batch's
// decision record reuses, so the audit payload does not introduce a new number.
const (
	maxResourceIDLength = 255
	maxUserAgentLength  = 255
	maxMetadataBytes    = 32 * 1024
)

// validateCreateRecordCommand runs before the repository write: the columns
// are NOT NULL at the database level, but an empty action/resourceType used to
// reach the INSERT and turn a validation mistake into a 201 with unusable
// audit evidence.
func validateCreateRecordCommand(command CreateRecordCommand) error {
	if strings.TrimSpace(command.Action) == "" {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"action is required",
			map[string]any{metaKeyField: "action"},
		)
	}
	if strings.TrimSpace(command.ResourceType) == "" {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"resourceType is required",
			map[string]any{metaKeyField: "resourceType"},
		)
	}
	if length := utf8.RuneCountInString(command.ResourceID); length > maxResourceIDLength {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"resourceId must not exceed 255 characters",
			map[string]any{metaKeyField: "resourceId", metaKeyValue: length, metaKeyMax: maxResourceIDLength},
		)
	}
	if length := utf8.RuneCountInString(command.UserAgent); length > maxUserAgentLength {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"userAgent must not exceed 255 characters",
			map[string]any{metaKeyField: "userAgent", metaKeyValue: length, metaKeyMax: maxUserAgentLength},
		)
	}
	if command.Metadata == nil {
		return nil
	}

	payload, err := json.Marshal(command.Metadata)
	if err != nil {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"metadata must be JSON-serializable",
			map[string]any{metaKeyField: "metadata"},
		)
	}
	if len(payload) > maxMetadataBytes {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"metadata must not exceed 32768 bytes",
			map[string]any{metaKeyField: "metadata", metaKeyValue: len(payload), metaKeyMax: maxMetadataBytes},
		)
	}
	return nil
}
