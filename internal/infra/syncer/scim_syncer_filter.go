package syncer

import (
	"fmt"
	"strings"
)

func ParseSCIMFilter(filter string) []SCIMFilterCondition {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return nil
	}

	// Split by " and " (case-insensitive) for AND conditions
	parts := splitSCIMFilterAND(filter)
	conditions := make([]SCIMFilterCondition, 0, len(parts))

	for _, part := range parts {
		cond := parseSingleFilter(strings.TrimSpace(part))
		if cond != nil {
			conditions = append(conditions, *cond)
		}
	}

	return conditions
}

// SCIMFilterCondition represents a single SCIM filter condition.
type SCIMFilterCondition struct {
	AttributePath string      // e.g., "userName", "emails.value"
	Operator      string      // eq, ne, co, sw, ew, gt, lt, ge, le, pr
	Value         interface{} // string, bool, int
}

// parseSingleFilter parses a single SCIM filter expression.
func parseSingleFilter(expr string) *SCIMFilterCondition {
	// Supported operators in order of length (longest first to avoid partial matches)
	operators := []string{"ge", "le", "gt", "lt", "ne", "eq", "co", "sw", "ew", "pr"}

	for _, op := range operators {
		// Look for " op " pattern
		searchStr := " " + op + " "
		idx := strings.Index(strings.ToLower(expr), searchStr)
		if idx == -1 {
			continue
		}

		attrPath := strings.TrimSpace(expr[:idx])
		valueStr := strings.TrimSpace(expr[idx+len(searchStr):])

		cond := &SCIMFilterCondition{
			AttributePath: attrPath,
			Operator:      op,
		}

		if op == "pr" {
			// "pr" (present) operator has no value
			cond.Value = nil
			return cond
		}

		// Parse the value
		cond.Value = parseSCIMValue(valueStr)
		return cond
	}

	return nil
}

// parseSCIMValue parses a SCIM filter value (string, bool, int, or null).
func parseSCIMValue(s string) interface{} {
	s = strings.TrimSpace(s)

	// Null
	if s == "null" {
		return nil
	}

	// Boolean
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Quoted string
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}

	// Number
	if strings.Contains(s, ".") {
		var f float64
		if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
			return f
		}
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
		return n
	}

	// Return as unquoted string
	return s
}

// splitSCIMFilterAND splits a SCIM filter by " and " while respecting quoted strings.
func splitSCIMFilterAND(filter string) []string {
	var parts []string
	inQuote := false
	start := 0

	for i := 0; i < len(filter); i++ {
		if filter[i] == '"' {
			inQuote = !inQuote
			continue
		}
		if !inQuote && i+5 <= len(filter) &&
			strings.ToLower(filter[i:i+5]) == " and " {
			parts = append(parts, filter[start:i])
			i += 4 // skip " and"
			start = i + 1
		}
	}

	if start < len(filter) {
		parts = append(parts, filter[start:])
	}

	return parts
}

// --- Helper Methods ---
