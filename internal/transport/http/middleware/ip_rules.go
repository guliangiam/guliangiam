package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/guliangiam/guliangiam/internal/transport/http/util"
)

// RuleEntry contains rule data needed by middleware, decoupled from domain types.
type RuleEntry struct {
	Name       string
	Action     string
	IsEnabled  bool
	Conditions []map[string]any
}

// RuleQueryService defines the interface for querying rules.
type RuleQueryService interface {
	ListEnabledByType(ctx context.Context, ruleType string) ([]RuleEntry, error)
}

// IPRulesMiddleware checks requests against IP blacklist/whitelist rules.
type IPRulesMiddleware struct {
	ruleSvc RuleQueryService
}

// NewIPRulesMiddleware creates a new IP rules middleware.
func NewIPRulesMiddleware(ruleSvc RuleQueryService) *IPRulesMiddleware {
	return &IPRulesMiddleware{
		ruleSvc: ruleSvc,
	}
}

// Middleware returns an HTTP middleware that checks IP rules.
func (m *IPRulesMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := util.ClientIP(r)

		ctx := r.Context()
		blocked, err := m.isIPBlocked(ctx, clientIP)
		if err != nil {
			// On error, allow the request (fail open for availability)
			next.ServeHTTP(w, r)
			return
		}

		if blocked {
			route.WriteError(w, r, apperrors.NewForbidden("access denied by IP rule"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isIPBlocked checks if an IP is blocked by any rule.
// Whitelist (Allow) rules take precedence over blacklist (Block) rules.
func (m *IPRulesMiddleware) isIPBlocked(ctx context.Context, ip string) (bool, error) {
	ipRules, err := m.ruleSvc.ListEnabledByType(ctx, "IP")
	if err != nil {
		return false, apperrors.WrapInternalf(err, "list IP rules")
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false, nil // Invalid IP, don't block
	}

	inWhitelist := false
	inBlacklist := false

	for _, rule := range ipRules {
		matched := matchRuleConditions(parsedIP, ip, rule.Conditions)

		if matched {
			switch rule.Action {
			case "Allow":
				inWhitelist = true
			case "Block":
				inBlacklist = true
			}
		}
	}

	// Whitelist takes precedence
	if inWhitelist {
		return false, nil
	}

	return inBlacklist, nil
}

// matchRuleConditions checks if an IP matches any condition in the rule.
func matchRuleConditions(parsedIP net.IP, rawIP string, conditions []map[string]any) bool {
	for _, cond := range conditions {
		field, _ := cond["field"].(string)
		op, _ := cond["operator"].(string)
		value, _ := cond["value"].(string)

		if field == "ip" {
			switch op {
			case "eq":
				if value == rawIP {
					return true
				}
			case "cidr":
				_, cidrNet, err := net.ParseCIDR(value)
				if err == nil && cidrNet.Contains(parsedIP) {
					return true
				}
			case "range":
				if matchIPRange(parsedIP, value) {
					return true
				}
			}
		}
	}
	return false
}

// matchIPRange checks if an IP falls within an inclusive range
// (e.g., "10.0.0.1-10.0.0.255" or "2001:db8::1-2001:db8::ffff").
// BE-E-04: netip whole-address comparison replaces the old octet-wise loop,
// which produced false negatives across octet boundaries and never matched
// IPv6. IPv4-mapped IPv6 addresses are unmapped so both families interoperate.
func matchIPRange(ip net.IP, ipRange string) bool {
	parts := strings.SplitN(ipRange, "-", 2)
	if len(parts) != 2 {
		return false
	}

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	start, err := netip.ParseAddr(strings.TrimSpace(parts[0]))
	if err != nil {
		return false
	}
	end, err := netip.ParseAddr(strings.TrimSpace(parts[1]))
	if err != nil {
		return false
	}

	addr, start, end = addr.Unmap(), start.Unmap(), end.Unmap()
	if addr.BitLen() != start.BitLen() || start.BitLen() != end.BitLen() {
		return false
	}
	return start.Compare(addr) <= 0 && addr.Compare(end) <= 0
}
