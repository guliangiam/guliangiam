package resources

import (
	"strings"

	enumset "github.com/guliangiam/guliangiam/internal/platform/enumset"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ResourceTypes is the closed set of resource types (手册 4.14.3).
//
// The values are produced by `classifyMimeType` in upload_resource.go, so this
// set is the declaration side of a value the server itself generates. It moved
// from a local `map[string]bool` to `enumset` so the same declaration can be
// compared against the console's option list by `scripts/check-enums.py`.
var ResourceTypes = enumset.NewSet(
	"resourceType",
	"image",
	"css",
	"js",
	"file",
	"video",
	"audio",
)

func validateCreateResourceCommand(command CreateResourceCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName is required")
	}
	if command.Type != "" && !ResourceTypes.Contains(command.Type) {
		return apperrors.NewInvalidArgument("invalid type")
	}
	return nil
}

func validatePatchResourceCommand(command PatchResourceCommand) error {
	if command.DisplayName != nil && strings.TrimSpace(*command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName must not be blank")
	}
	if command.Type != nil && !ResourceTypes.Contains(*command.Type) {
		return apperrors.NewInvalidArgument("invalid type")
	}
	return nil
}
