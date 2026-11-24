package middleware

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchRuleConditions_IP_EQ(t *testing.T) {
	parsedIP := net.ParseIP("192.168.1.1")
	conditions := []map[string]any{
		{"field": "ip", "operator": "eq", "value": "192.168.1.1"},
	}
	assert.True(t, matchRuleConditions(parsedIP, "192.168.1.1", conditions))
}

func TestMatchRuleConditions_IP_EQ_NoMatch(t *testing.T) {
	parsedIP := net.ParseIP("192.168.1.1")
	conditions := []map[string]any{
		{"field": "ip", "operator": "eq", "value": "10.0.0.1"},
	}
	assert.False(t, matchRuleConditions(parsedIP, "192.168.1.1", conditions))
}

func TestMatchRuleConditions_IP_CIDR(t *testing.T) {
	parsedIP := net.ParseIP("192.168.1.100")
	conditions := []map[string]any{
		{"field": "ip", "operator": "cidr", "value": "192.168.1.0/24"},
	}
	assert.True(t, matchRuleConditions(parsedIP, "192.168.1.100", conditions))
}

func TestMatchRuleConditions_IP_CIDR_NoMatch(t *testing.T) {
	parsedIP := net.ParseIP("10.0.0.1")
	conditions := []map[string]any{
		{"field": "ip", "operator": "cidr", "value": "192.168.1.0/24"},
	}
	assert.False(t, matchRuleConditions(parsedIP, "10.0.0.1", conditions))
}

func TestMatchRuleConditions_IP_CIDR_InvalidCIDR(t *testing.T) {
	parsedIP := net.ParseIP("192.168.1.1")
	conditions := []map[string]any{
		{"field": "ip", "operator": "cidr", "value": "not-a-cidr"},
	}
	assert.False(t, matchRuleConditions(parsedIP, "192.168.1.1", conditions))
}

func TestMatchRuleConditions_IP_Range(t *testing.T) {
	parsedIP := net.ParseIP("192.168.1.50")
	conditions := []map[string]any{
		{"field": "ip", "operator": "range", "value": "192.168.1.1-192.168.1.255"},
	}
	assert.True(t, matchRuleConditions(parsedIP, "192.168.1.50", conditions))
}

func TestMatchRuleConditions_IP_Range_NoMatch(t *testing.T) {
	parsedIP := net.ParseIP("10.0.0.1")
	conditions := []map[string]any{
		{"field": "ip", "operator": "range", "value": "192.168.1.1-192.168.1.255"},
	}
	assert.False(t, matchRuleConditions(parsedIP, "10.0.0.1", conditions))
}

func TestMatchRuleConditions_UnknownField(t *testing.T) {
	parsedIP := net.ParseIP("192.168.1.1")
	conditions := []map[string]any{
		{"field": "country", "operator": "eq", "value": "US"},
	}
	assert.False(t, matchRuleConditions(parsedIP, "192.168.1.1", conditions))
}

func TestMatchRuleConditions_UnknownOperator(t *testing.T) {
	parsedIP := net.ParseIP("192.168.1.1")
	conditions := []map[string]any{
		{"field": "ip", "operator": "regex", "value": "192.*"},
	}
	assert.False(t, matchRuleConditions(parsedIP, "192.168.1.1", conditions))
}

func TestMatchRuleConditions_EmptyConditions(t *testing.T) {
	parsedIP := net.ParseIP("192.168.1.1")
	assert.False(t, matchRuleConditions(parsedIP, "192.168.1.1", []map[string]any{}))
}

func TestMatchRuleConditions_MultipleConditions(t *testing.T) {
	parsedIP := net.ParseIP("192.168.1.1")
	conditions := []map[string]any{
		{"field": "ip", "operator": "eq", "value": "10.0.0.1"},         // no match
		{"field": "ip", "operator": "cidr", "value": "192.168.1.0/24"}, // match
	}
	assert.True(t, matchRuleConditions(parsedIP, "192.168.1.1", conditions))
}

func TestMatchIPRange_Valid(t *testing.T) {
	ip := net.ParseIP("192.168.1.100")
	assert.True(t, matchIPRange(ip, "192.168.1.1-192.168.1.255"))
}

func TestMatchIPRange_StartIP(t *testing.T) {
	ip := net.ParseIP("192.168.1.1")
	assert.True(t, matchIPRange(ip, "192.168.1.1-192.168.1.255"))
}

func TestMatchIPRange_EndIP(t *testing.T) {
	ip := net.ParseIP("192.168.1.255")
	assert.True(t, matchIPRange(ip, "192.168.1.1-192.168.1.255"))
}

func TestMatchIPRange_OutOfRange(t *testing.T) {
	ip := net.ParseIP("192.168.2.1")
	assert.False(t, matchIPRange(ip, "192.168.1.1-192.168.1.255"))
}

func TestMatchIPRange_InvalidFormat(t *testing.T) {
	ip := net.ParseIP("192.168.1.1")
	assert.False(t, matchIPRange(ip, "not-a-range"))
}

func TestMatchIPRange_InvalidStartIP(t *testing.T) {
	ip := net.ParseIP("192.168.1.1")
	assert.False(t, matchIPRange(ip, "invalid-192.168.1.255"))
}

func TestMatchIPRange_InvalidEndIP(t *testing.T) {
	ip := net.ParseIP("192.168.1.1")
	assert.False(t, matchIPRange(ip, "192.168.1.1-invalid"))
}

func TestMatchIPRange_IPv6(t *testing.T) {
	// BE-E-04: IPv6 ranges are now supported (the old octet loop always
	// returned false for IPv6).
	ip := net.ParseIP("::1")
	assert.True(t, matchIPRange(ip, "::1-::ffff"))
	assert.False(t, matchIPRange(net.ParseIP("::1:0"), "::1-::ffff"))
}

func TestMatchIPRange_CrossOctetBoundary(t *testing.T) {
	// BE-E-04 regression: the old octet-wise loop rejected addresses whose
	// upper octet equalled the start while a lower octet exceeded the end
	// octet, even though the address is inside the range.
	rng := "10.0.0.200-10.0.1.50"
	assert.True(t, matchIPRange(net.ParseIP("10.0.0.250"), rng))
	assert.True(t, matchIPRange(net.ParseIP("10.0.1.0"), rng))
	assert.True(t, matchIPRange(net.ParseIP("10.0.1.50"), rng))
	assert.False(t, matchIPRange(net.ParseIP("10.0.0.199"), rng))
	assert.False(t, matchIPRange(net.ParseIP("10.0.1.51"), rng))
	assert.False(t, matchIPRange(net.ParseIP("10.0.2.0"), rng))
}

func TestMatchIPRange_IPv6Wide(t *testing.T) {
	rng := "2001:db8::1-2001:db8::ffff"
	assert.True(t, matchIPRange(net.ParseIP("2001:db8::abcd"), rng))
	assert.False(t, matchIPRange(net.ParseIP("2001:db8::1:0"), rng))
	assert.False(t, matchIPRange(net.ParseIP("2001:db7::ffff"), rng))
}

func TestMatchIPRange_MixedFamily(t *testing.T) {
	assert.False(t, matchIPRange(net.ParseIP("10.0.0.5"), "::1-::ffff"))
	assert.False(t, matchIPRange(net.ParseIP("::5"), "10.0.0.1-10.0.0.255"))
	// IPv4-mapped IPv6 unmaps to the v4 family and matches.
	assert.True(t, matchIPRange(net.ParseIP("::ffff:10.0.0.5"), "10.0.0.1-10.0.0.255"))
}

func TestMatchIPRange_ReversedRange(t *testing.T) {
	assert.False(t, matchIPRange(net.ParseIP("10.0.0.5"), "10.0.0.255-10.0.0.1"))
}

func TestIPRulesMiddleware_isIPBlocked_InvalidIP(t *testing.T) {
	m := &IPRulesMiddleware{ruleSvc: &mockRuleQueryService{}}
	blocked, err := m.isIPBlocked(context.Background(), "not-an-ip")
	// nil context is ok for the mock
	_ = blocked
	_ = err
}

func TestIPRulesMiddleware_isIPBlocked_InvalidIPString(t *testing.T) {
	m := &IPRulesMiddleware{ruleSvc: &mockRuleQueryService{rules: []RuleEntry{}}}
	blocked, err := m.isIPBlocked(context.Background(), "not-an-ip")
	assert.NoError(t, err)
	assert.False(t, blocked)
}

type mockRuleQueryService struct {
	rules []RuleEntry
	err   error
}

func (m *mockRuleQueryService) ListEnabledByType(ctx context.Context, ruleType string) ([]RuleEntry, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.rules, nil
}

func TestIPRulesMiddleware_isIPBlocked_WhitelistPrecedence(t *testing.T) {
	m := &IPRulesMiddleware{ruleSvc: &mockRuleQueryService{
		rules: []RuleEntry{
			{Name: "block-all", Action: "Block", IsEnabled: true, Conditions: []map[string]any{
				{"field": "ip", "operator": "cidr", "value": "0.0.0.0/0"},
			}},
			{Name: "allow-specific", Action: "Allow", IsEnabled: true, Conditions: []map[string]any{
				{"field": "ip", "operator": "eq", "value": "192.168.1.1"},
			}},
		},
	}}
	blocked, err := m.isIPBlocked(context.Background(), "192.168.1.1")
	assert.NoError(t, err)
	assert.False(t, blocked) // Whitelist takes precedence
}

func TestIPRulesMiddleware_isIPBlocked_BlacklistOnly(t *testing.T) {
	m := &IPRulesMiddleware{ruleSvc: &mockRuleQueryService{
		rules: []RuleEntry{
			{Name: "block-specific", Action: "Block", IsEnabled: true, Conditions: []map[string]any{
				{"field": "ip", "operator": "eq", "value": "10.0.0.1"},
			}},
		},
	}}
	blocked, err := m.isIPBlocked(context.Background(), "10.0.0.1")
	assert.NoError(t, err)
	assert.True(t, blocked)
}

func TestIPRulesMiddleware_isIPBlocked_NoMatch(t *testing.T) {
	m := &IPRulesMiddleware{ruleSvc: &mockRuleQueryService{
		rules: []RuleEntry{
			{Name: "block-specific", Action: "Block", IsEnabled: true, Conditions: []map[string]any{
				{"field": "ip", "operator": "eq", "value": "10.0.0.1"},
			}},
		},
	}}
	blocked, err := m.isIPBlocked(context.Background(), "192.168.1.1")
	assert.NoError(t, err)
	assert.False(t, blocked)
}

func TestIPRulesMiddleware_isIPBlocked_QueryError(t *testing.T) {
	m := &IPRulesMiddleware{ruleSvc: &mockRuleQueryService{
		err: assert.AnError,
	}}
	_, err := m.isIPBlocked(context.Background(), "192.168.1.1")
	assert.Error(t, err)
}

func TestNewIPRulesMiddleware(t *testing.T) {
	m := NewIPRulesMiddleware(&mockRuleQueryService{})
	assert.NotNil(t, m)
}
