package auth

import (
	"context"
	"log/slog"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BreachChecker reports whether a password appears in a breached-password
// corpus (AUTH-009). Implementations must keep the plaintext and the full
// hash local: the HIBP k-anonymity protocol sends only the 5-char SHA-1
// prefix out of the process.
type BreachChecker interface {
	IsBreached(ctx context.Context, password string) (bool, error)
}

// checkPasswordBreach 在注册/改密/重置三条路径的策略校验之后执行。
// 依赖不可用时 fail-open（用户裁定，G4-ab）：放行并返回 unverified=true，
// 由调用方在审计 payload 标记 breachCheck=unverified；命中泄露库返回 422。
// BreachChecker 为 nil（开关默认关）时直接放行。
func (s *Service) checkPasswordBreach(ctx context.Context, password string) (unverified bool, err error) {
	if s.BreachChecker == nil {
		return false, nil
	}
	breached, err := s.BreachChecker.IsBreached(ctx, password)
	if err != nil {
		slog.WarnContext(ctx, "password breach check unavailable; failing open", "error", err)
		return true, nil
	}
	if breached {
		return false, apperrors.NewUnprocessableEntity("password has appeared in a data breach; choose a different password")
	}
	return false, nil
}

// withBreachAuditMarker 在 fail-open 放行时给审计 payload 打 unverified 标记。
func withBreachAuditMarker(payload map[string]any, unverified bool) map[string]any {
	if unverified {
		payload["breachCheck"] = "unverified"
	}
	return payload
}
