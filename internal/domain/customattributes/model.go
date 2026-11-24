// Package customattributes defines the tenant-scoped schema for custom user
// profile properties (OPEN-API-59). Definitions decide which keys may appear in
// user_profiles.properties and how their values are validated.
package customattributes

import (
	"context"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DataType is the closed set of supported property value types.
type DataType string

const (
	DataTypeString  DataType = "string"
	DataTypeNumber  DataType = "number"
	DataTypeBoolean DataType = "boolean"
	DataTypeDate    DataType = "date"
	DataTypeEnum    DataType = "enum"
)

// Visibility controls who may read the property value.
type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityPublic  Visibility = "public"
	VisibilityAdmin   Visibility = "admin"
)

var keyPattern = regexp.MustCompile("^[a-z][a-z0-9_.-]{0,63}$")

// Definition is one tenant-scoped custom attribute declaration.
type Definition struct {
	ID             string
	OrganizationID string
	Key            string
	DataType       DataType
	Required       bool
	Options        []string
	Writable       bool
	Visibility     Visibility
	Description    string
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// IsValidDataType reports whether t is a supported data type.
func IsValidDataType(t DataType) bool {
	switch t {
	case DataTypeString, DataTypeNumber, DataTypeBoolean, DataTypeDate, DataTypeEnum:
		return true
	default:
		return false
	}
}

// IsValidVisibility reports whether v is a supported visibility level.
func IsValidVisibility(v Visibility) bool {
	switch v {
	case VisibilityPrivate, VisibilityPublic, VisibilityAdmin:
		return true
	default:
		return false
	}
}

// Validate checks a definition before persistence. Enum definitions require a
// non-empty option set; non-enum definitions must not carry options.
func (d Definition) Validate() error {
	if d.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	key := strings.TrimSpace(d.Key)
	if !keyPattern.MatchString(key) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"attribute key must match ^[a-z][a-z0-9_.-]{0,63}$",
			map[string]any{"field": "key", "value": d.Key})
	}
	if !IsValidDataType(d.DataType) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"unsupported attribute data type",
			map[string]any{"field": "dataType", "value": string(d.DataType)})
	}
	if !IsValidVisibility(d.Visibility) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			"unsupported attribute visibility",
			map[string]any{"field": "visibility", "value": string(d.Visibility)})
	}
	switch d.DataType {
	case DataTypeEnum:
		if len(d.Options) == 0 {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"enum attributes require at least one option",
				map[string]any{"field": "options"})
		}
		seen := map[string]bool{}
		for _, option := range d.Options {
			trimmed := strings.TrimSpace(option)
			if trimmed == "" {
				return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
					"enum options must not contain blank values", map[string]any{"field": "options"})
			}
			if seen[trimmed] {
				return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
					"enum options must be unique", map[string]any{"field": "options", "value": trimmed})
			}
			seen[trimmed] = true
		}
	default:
		if len(d.Options) > 0 {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"options are only allowed for enum attributes", map[string]any{"field": "options"})
		}
	}
	return nil
}

// Repository is the persistence port for definitions. Every read and write is
// tenant-scoped; callers pass the trusted organization id.
type Repository interface {
	ListByOrganization(ctx context.Context, organizationID string, includeDisabled bool) ([]*Definition, error)
	GetByOrganizationAndKey(ctx context.Context, organizationID, key string) (*Definition, error)
	Create(ctx context.Context, definition *Definition) error
	Update(ctx context.Context, definition *Definition) error
	Delete(ctx context.Context, organizationID, id string) error
}
