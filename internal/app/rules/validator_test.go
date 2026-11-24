package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleValidator_Validate(t *testing.T) {
	v := RuleValidator{}

	t.Run("waf_valid", func(t *testing.T) {
		result := v.Validate("waf", []string{"SecRule ARGS \"<script>\" \"deny\""})
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
	})

	t.Run("waf_empty", func(t *testing.T) {
		result := v.Validate("waf", []string{})
		assert.False(t, result.Valid)
	})

	t.Run("waf_dangerous_exec", func(t *testing.T) {
		result := v.Validate("waf", []string{"SecRule ARGS \"exec\" \"deny\""})
		assert.False(t, result.Valid)
	})

	t.Run("waf_dangerous_system", func(t *testing.T) {
		result := v.Validate("waf", []string{"SecRule ARGS \"System\" \"deny\""})
		assert.False(t, result.Valid)
	})

	t.Run("waf_empty_expression", func(t *testing.T) {
		result := v.Validate("waf", []string{"  "})
		assert.False(t, result.Valid)
	})

	t.Run("ip_valid_ipv4", func(t *testing.T) {
		result := v.Validate("ip", []string{"192.168.1.1"})
		assert.True(t, result.Valid)
	})

	t.Run("ip_valid_ipv6", func(t *testing.T) {
		result := v.Validate("ip", []string{"::1"})
		assert.True(t, result.Valid)
	})

	t.Run("ip_valid_cidr", func(t *testing.T) {
		result := v.Validate("ip", []string{"10.0.0.0/8"})
		assert.True(t, result.Valid)
	})

	t.Run("ip_invalid", func(t *testing.T) {
		result := v.Validate("ip", []string{"not-an-ip"})
		assert.False(t, result.Valid)
	})

	t.Run("ip_invalid_cidr", func(t *testing.T) {
		result := v.Validate("ip", []string{"10.0.0.0/33"})
		assert.False(t, result.Valid)
	})

	t.Run("ip_empty", func(t *testing.T) {
		result := v.Validate("ip", []string{})
		assert.False(t, result.Valid)
	})

	t.Run("ip_empty_expression", func(t *testing.T) {
		result := v.Validate("ip", []string{"  "})
		assert.False(t, result.Valid)
	})

	t.Run("rate_limit_valid", func(t *testing.T) {
		result := v.Validate("rate_limit", []string{"100/1m"})
		assert.True(t, result.Valid)
	})

	t.Run("rate_limit_valid_seconds", func(t *testing.T) {
		result := v.Validate("rate_limit", []string{"50/30s"})
		assert.True(t, result.Valid)
	})

	t.Run("rate_limit_valid_hours", func(t *testing.T) {
		result := v.Validate("rate_limit", []string{"1000/1h"})
		assert.True(t, result.Valid)
	})

	t.Run("rate_limit_invalid_format", func(t *testing.T) {
		result := v.Validate("rate_limit", []string{"100 per minute"})
		assert.False(t, result.Valid)
	})

	t.Run("rate_limit_empty", func(t *testing.T) {
		result := v.Validate("rate_limit", []string{})
		assert.False(t, result.Valid)
	})

	t.Run("rate_limit_empty_expression", func(t *testing.T) {
		result := v.Validate("rate_limit", []string{"  "})
		assert.False(t, result.Valid)
	})

	t.Run("compound_valid", func(t *testing.T) {
		result := v.Validate("compound", []string{"rule-1", "rule-2"})
		assert.True(t, result.Valid)
	})

	t.Run("compound_empty", func(t *testing.T) {
		result := v.Validate("compound", []string{})
		assert.False(t, result.Valid)
	})

	t.Run("compound_empty_expression", func(t *testing.T) {
		result := v.Validate("compound", []string{"  "})
		assert.False(t, result.Valid)
	})

	t.Run("unknown_type_with_conditions", func(t *testing.T) {
		result := v.Validate("custom", []string{"some-condition"})
		assert.True(t, result.Valid)
	})

	t.Run("unknown_type_no_conditions", func(t *testing.T) {
		result := v.Validate("custom", []string{})
		assert.False(t, result.Valid)
	})

	t.Run("case_insensitive_type", func(t *testing.T) {
		result := v.Validate("WAF", []string{"SecRule ARGS \"test\" \"deny\""})
		assert.True(t, result.Valid)
	})
}
