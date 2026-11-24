package scim

import (

	// stdlib errors retained for errors.Is/errors.As only; apperrors used for error creation
	"fmt"
	"strings"
)

func (r *Repository) buildFilterWhere(filter string, startPos int) whereClause {
	// SCIM filter supports: eq, ne, sw, ew, co
	// Format: <attribute> <op> "<value>"
	// Examples: userName eq "admin", displayName co "test", name sw "Dev"

	// Try to parse SCIM filter: attribute op "value"
	for _, op := range []string{" eq ", " ne ", " sw ", " ew ", " co "} {
		idx := indexOfCaseInsensitive(filter, op)
		if idx < 0 {
			continue
		}

		attr := strings.TrimSpace(filter[:idx])
		value := strings.TrimSpace(filter[idx+len(op):])
		// Strip surrounding quotes
		value = strings.Trim(value, `"'`)

		// Map SCIM attribute to DB column
		column := scimAttrToColumn(attr)
		if column == "" {
			return whereClause{
				clause:  "1 = 0",
				args:    []interface{}{},
				nextPos: startPos,
			}
		}

		switch strings.TrimSpace(op) {
		case "eq":
			return whereClause{
				clause:  fmt.Sprintf("%s = $%d", column, startPos),
				args:    []interface{}{value},
				nextPos: startPos + 1,
			}
		case "ne":
			return whereClause{
				clause:  fmt.Sprintf("%s != $%d", column, startPos),
				args:    []interface{}{value},
				nextPos: startPos + 1,
			}
		case "sw":
			return whereClause{
				clause:  fmt.Sprintf("%s ILIKE $%d", column, startPos),
				args:    []interface{}{value + "%"},
				nextPos: startPos + 1,
			}
		case "ew":
			return whereClause{
				clause:  fmt.Sprintf("%s ILIKE $%d", column, startPos),
				args:    []interface{}{"%" + value},
				nextPos: startPos + 1,
			}
		case "co":
			return whereClause{
				clause:  fmt.Sprintf("%s ILIKE $%d", column, startPos),
				args:    []interface{}{"%" + value + "%"},
				nextPos: startPos + 1,
			}
		}
	}

	// Fallback: simple name contains filter
	return whereClause{
		clause:  fmt.Sprintf("display_name ILIKE $%d", startPos),
		args:    []interface{}{"%" + filter + "%"},
		nextPos: startPos + 1,
	}
}

// scimAttrToColumn maps SCIM attribute names to database column names.
func scimAttrToColumn(attr string) string {
	switch attr {
	case attrUserName:
		return "username"
	case attrDisplayName:
		return "display_name"
	case attrName:
		return "name"
	case attrEmail:
		return "email"
	case attrPhone:
		return "phone"
	case attrActive:
		return "status"
	default:
		return ""
	}
}

// indexOfCaseInsensitive finds the index of a substring, case-insensitive.
func indexOfCaseInsensitive(s, substr string) int {
	sLower := strings.ToLower(s)
	subLower := strings.ToLower(substr)
	return strings.Index(sLower, subLower)
}
