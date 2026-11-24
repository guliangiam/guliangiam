package rules

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// ValidationResult contains the result of rule expression validation.
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// RuleValidator validates rule expressions based on their type.
type RuleValidator struct{}

// Validate validates rule conditions/expressions based on ruleType.
func (v RuleValidator) Validate(ruleType string, conditions []string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	switch strings.ToLower(ruleType) {
	case "waf":
		v.validateWAF(conditions, result)
	case "ip":
		v.validateIP(conditions, result)
	case "rate_limit", "rate":
		v.validateRateLimit(conditions, result)
	case "compound":
		// Compound rules reference other rules; basic format check only
		v.validateCompound(conditions, result)
	default:
		// Unknown type: allow but warn
		if len(conditions) == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, "at least one condition is required")
		}
	}

	return result
}

// validateWAF performs basic validation on WAF/ModSecurity expressions.
func (v RuleValidator) validateWAF(expressions []string, result *ValidationResult) {
	if len(expressions) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "at least one WAF expression is required")
		return
	}

	for i, expr := range expressions {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("expression %d is empty", i+1))
			continue
		}
		// Basic sanity check: reject obviously dangerous directives
		lower := strings.ToLower(expr)
		if strings.Contains(lower, "exec") || strings.Contains(lower, "system") {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("expression %d contains dangerous directive", i+1))
		}
	}
}

// validateIP validates that each expression is a valid IP address or CIDR.
func (v RuleValidator) validateIP(expressions []string, result *ValidationResult) {
	if len(expressions) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "at least one IP/CIDR expression is required")
		return
	}

	for i, expr := range expressions {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("expression %d is empty", i+1))
			continue
		}
		// Try parsing as CIDR first, then as plain IP
		if strings.Contains(expr, "/") {
			_, _, err := net.ParseCIDR(expr)
			if err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("expression %d is not a valid CIDR: %s", i+1, expr))
			}
		} else {
			ip := net.ParseIP(expr)
			if ip == nil {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("expression %d is not a valid IP address: %s", i+1, expr))
			}
		}
	}
}

var rateLimitPattern = regexp.MustCompile(`^\d+/\d+[smh]$`)

// validateRateLimit validates rate limit expressions (format: "requests/period" like "100/1m").
func (v RuleValidator) validateRateLimit(expressions []string, result *ValidationResult) {
	if len(expressions) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "at least one rate limit expression is required")
		return
	}

	for i, expr := range expressions {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("expression %d is empty", i+1))
			continue
		}
		if !rateLimitPattern.MatchString(expr) {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("expression %d has invalid format (expected 'N/T' where T is e.g. '1s', '5m', '1h'): %s", i+1, expr))
		}
	}
}

// validateCompound validates compound rule references.
func (v RuleValidator) validateCompound(expressions []string, result *ValidationResult) {
	if len(expressions) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "at least one compound expression is required")
		return
	}
	// Basic format: each expression should be a rule reference (UUID or name)
	for i, expr := range expressions {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("expression %d is empty", i+1))
		}
	}
}
