package customattributes

import (
	"fmt"
	"math"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ValidateValue checks one property value against its definition. present
// distinguishes "absent" from an explicit null/empty value.
func ValidateValue(definition Definition, value any, present bool) error {
	if !present {
		if definition.Required {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				fmt.Sprintf("property %q is required", definition.Key),
				map[string]any{"field": "properties", "key": definition.Key, "reason": "required"})
		}
		return nil
	}
	switch definition.DataType {
	case DataTypeString:
		if _, ok := value.(string); !ok {
			return invalidType(definition, "string")
		}
	case DataTypeNumber:
		if !isNumber(value) {
			return invalidType(definition, "number")
		}
	case DataTypeBoolean:
		if _, ok := value.(bool); !ok {
			return invalidType(definition, "boolean")
		}
	case DataTypeDate:
		text, ok := value.(string)
		if !ok || !parseDate(text) {
			return invalidType(definition, "date (RFC3339 or YYYY-MM-DD)")
		}
	case DataTypeEnum:
		text, ok := value.(string)
		if !ok {
			return invalidType(definition, "enum string")
		}
		for _, option := range definition.Options {
			if option == text {
				return nil
			}
		}
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("property %q is not one of the declared options", definition.Key),
			map[string]any{"field": "properties", "key": definition.Key, "allowed": definition.Options})
	default:
		return apperrors.NewInternal("unknown custom attribute data type")
	}
	return nil
}

func invalidType(definition Definition, expected string) error {
	return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
		fmt.Sprintf("property %q must be a %s", definition.Key, expected),
		map[string]any{"field": "properties", "key": definition.Key, "expected": expected})
}

func isNumber(value any) bool {
	switch typed := value.(type) {
	case int, int32, int64:
		return true
	case float32:
		return !float32IsNaNOrInf(typed)
	case float64:
		return !math.IsNaN(typed) && !math.IsInf(typed, 0)
	default:
		return false
	}
}

func float32IsNaNOrInf(value float32) bool {
	f := float64(value)
	return math.IsNaN(f) || math.IsInf(f, 0)
}

func parseDate(raw string) bool {
	text := strings.TrimSpace(raw)
	if text == "" {
		return false
	}
	if _, err := time.Parse(time.RFC3339, text); err == nil {
		return true
	}
	_, err := time.Parse("2006-01-02", text)
	return err == nil
}

// ValidateProperties applies every definition to the submitted property map:
// unregistered keys are rejected, disabled definitions are rejected, and
// required definitions must be present. Definitions not mentioned in the map
// still enforce their required flag.
func ValidateProperties(properties map[string]any, definitions []Definition) error {
	byKey := make(map[string]Definition, len(definitions))
	for _, definition := range definitions {
		byKey[definition.Key] = definition
	}
	for key, value := range properties {
		definition, ok := byKey[key]
		if !ok {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				fmt.Sprintf("property %q is not declared for this organization", key),
				map[string]any{"field": "properties", "key": key, "reason": "unregistered"})
		}
		if !definition.Enabled {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				fmt.Sprintf("property %q is disabled", key),
				map[string]any{"field": "properties", "key": key, "reason": "disabled"})
		}
		if !definition.Writable {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				fmt.Sprintf("property %q is read-only", key),
				map[string]any{"field": "properties", "key": key, "reason": "read_only"})
		}
		if err := ValidateValue(definition, value, true); err != nil {
			return err
		}
	}
	for _, definition := range definitions {
		if _, present := properties[definition.Key]; present {
			continue
		}
		if err := ValidateValue(definition, nil, false); err != nil {
			return err
		}
	}
	return nil
}
