package workload

import (
	"fmt"
	"strings"
	"unicode"
)

func ValidateSubjectPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("subject pattern must not be empty")
	}
	if hasWhitespaceOrControl(pattern) {
		return fmt.Errorf("subject pattern must not contain whitespace or control characters")
	}
	wildcards := strings.Count(pattern, "*")
	if wildcards == 0 {
		return nil
	}
	if wildcards != 1 || !strings.HasSuffix(pattern, "/*") {
		return fmt.Errorf("subject pattern wildcard is only allowed as a trailing /*")
	}
	if strings.TrimSuffix(pattern, "/*") == "" {
		return fmt.Errorf("subject pattern wildcard must have a non-empty prefix")
	}
	return nil
}

func ValidateSubjectPatterns(patterns []string) error {
	if len(patterns) == 0 {
		return fmt.Errorf("at least one subject pattern is required")
	}
	for _, pattern := range patterns {
		if err := ValidateSubjectPattern(pattern); err != nil {
			return err
		}
	}
	return nil
}

func MatchSubject(patterns []string, subject string) bool {
	if subject == "" || hasWhitespaceOrControl(subject) {
		return false
	}
	if ValidateSubjectPatterns(patterns) != nil {
		return false
	}
	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "/*") {
			if strings.HasPrefix(subject, strings.TrimSuffix(pattern, "*")) {
				return true
			}
			continue
		}
		if subject == pattern {
			return true
		}
	}
	return false
}

func hasWhitespaceOrControl(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
	}) >= 0
}
