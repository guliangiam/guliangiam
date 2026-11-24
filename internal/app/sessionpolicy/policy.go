package sessionpolicy

import (
	"context"
	"strings"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

// ClientBinding configures whether a session is bound to the client that
// created it. Both flags default to false (opt-in), so enabling a binding is
// an explicit per-organization decision rather than a silent behavior change.
type ClientBinding struct {
	IP        bool
	UserAgent bool
}

// Policy is the resolved session policy for one organization.
type Policy struct {
	IdleTimeout time.Duration
	Binding     ClientBinding
}

// ResolvePolicy loads the organization session policy once and returns the
// idle timeout plus client-binding flags. Callers on the request path should
// use this instead of two separate lookups.
//
// When the repository implements orgdomain.SessionPolicyReader the policy is
// read as a single narrow column; otherwise we fall back to the full aggregate
// so behavior is identical for repositories without the capability.
func ResolvePolicy(ctx context.Context, repository orgdomain.Repository, organizationID string) (Policy, error) {
	if organizationID == "" || repository == nil {
		return Policy{}, nil
	}

	raw := map[string]any(nil)
	if reader, ok := repository.(orgdomain.SessionPolicyReader); ok {
		policy, err := reader.GetSessionPolicy(ctx, organizationID)
		if err != nil {
			return Policy{}, err
		}
		raw = policy
	} else {
		organization, err := repository.GetByID(ctx, organizationID)
		if err != nil {
			return Policy{}, err
		}
		raw = organization.Security.SessionPolicy
	}

	var policy Policy
	if minutes, ok := numericMinutes(raw["idleTimeoutMinutes"]); ok && minutes > 0 {
		policy.IdleTimeout = time.Duration(minutes) * time.Minute
	}
	policy.Binding.IP = boolValue(raw["bindIp"])
	policy.Binding.UserAgent = boolValue(raw["bindUserAgent"])
	return policy, nil
}

// ResolveIdleTimeout is the idle-timeout-only view kept for callers that do
// not need client binding.
func ResolveIdleTimeout(ctx context.Context, repository orgdomain.Repository, organizationID string) (time.Duration, error) {
	policy, err := ResolvePolicy(ctx, repository, organizationID)
	if err != nil {
		return 0, err
	}
	return policy.IdleTimeout, nil
}

// ClientMismatch reports whether the request client differs from the session
// binding under an enabled policy. Empty values never trigger a mismatch, so
// internal callers that do not have request metadata keep working.
func ClientMismatch(sessionIP, sessionUserAgent, requestIP, requestUserAgent string, binding ClientBinding) bool {
	if binding.IP {
		a, b := strings.TrimSpace(sessionIP), strings.TrimSpace(requestIP)
		if a != "" && b != "" && a != b {
			return true
		}
	}
	if binding.UserAgent {
		a, b := strings.TrimSpace(sessionUserAgent), strings.TrimSpace(requestUserAgent)
		if a != "" && b != "" && a != b {
			return true
		}
	}
	return false
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func IsIdleExpired(now, lastActivityAt time.Time, idleTimeout time.Duration) bool {
	if idleTimeout <= 0 {
		return false
	}

	return !lastActivityAt.Add(idleTimeout).After(now)
}

func ShouldTouchActivity(now, lastActivityAt time.Time, idleTimeout time.Duration) bool {
	if idleTimeout <= 0 || !now.After(lastActivityAt) {
		return false
	}

	interval := 5 * time.Minute
	halfIdleTimeout := idleTimeout / 2
	if halfIdleTimeout > 0 && halfIdleTimeout < interval {
		interval = halfIdleTimeout
	}
	if interval < time.Minute {
		interval = time.Minute
	}

	return now.Sub(lastActivityAt) >= interval
}

func numericMinutes(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint:
		return int64(typed), true // #nosec G115 -- session timeout minutes never exceed int64 max
	case uint8:
		return int64(typed), true
	case uint16:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		return int64(typed), true // #nosec G115 -- session timeout minutes never exceed int64 max
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	default:
		return 0, false
	}
}
