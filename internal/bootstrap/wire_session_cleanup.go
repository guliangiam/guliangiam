// wire_session_cleanup.go — Session + WebAuthn Challenge 过期清理后台 goroutine。
// 从 runtime.go NewRuntime L462-490 抽离，保持 bootstrap 包级 private function。
package bootstrap

import (
	"context"
	"log/slog"
	"time"

	"github.com/guliangiam/guliangiam/internal/app/auth"
	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	domainidem "github.com/guliangiam/guliangiam/internal/domain/idempotency"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
)

// startSessionCleanup 启动后台清理 goroutine，返回停止 channel。
// 清理策略见 L463-464 原注释：默认 7 天过期，每小时执行一次。
func startSessionCleanup(authService auth.Service, sessionService appsessions.Service, idemRepo domainidem.Repository) chan struct{} {
	stopCh := make(chan struct{})
	interval := time.Duration(config.SessionCleanupIntervalMinutes()) * time.Minute
	retention := time.Duration(config.SessionCleanupRetentionHours()) * time.Hour
	recover.SafeGo("session-cleanup", func() {
		// One-time legacy invalidation at startup: rows written before the
		// SHA-256 digest switch can never match the login predicate, so revoke
		// them explicitly (idempotent) instead of relying on a silent miss.
		startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		if revoked, err := authService.RevokeLegacyPlaintextTrustedDevices(startupCtx, time.Now().UTC()); err != nil {
			slog.Warn("legacy trusted-device invalidation failed", "error", err)
		} else if revoked > 0 {
			slog.Info("legacy plaintext trusted devices revoked", "rows", revoked)
		}
		startupCancel()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		slog.Info("session cleanup started", "interval", interval, "retention", retention)
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if _, err := sessionService.CleanupExpiredSessions(ctx, retention); err != nil {
					slog.Warn("session cleanup failed", "error", err)
				}
				// WebAuthn 挑战同一节拍清理：过期行只占空间（finish 的消费谓词自带
				// expires_at > now()），保留期见 appauth.WebAuthnChallengeRetention。
				if _, err := authService.CleanupExpiredWebAuthnChallenges(ctx, auth.WebAuthnChallengeRetention); err != nil {
					slog.Warn("webauthn challenge cleanup failed", "error", err)
				}
				// ADR-018 TTL cleanup: idempotency rows are safe to drop once
				// expires_at has passed; bounded batches keep the delete short.
				if cleaner, ok := idemRepo.(domainidem.ExpiredCleaner); ok {
					var total int64
					for i := 0; i < 10; i++ {
						deleted, err := cleaner.DeleteExpired(ctx, time.Now().UTC(), 1000)
						if err != nil {
							slog.Warn("idempotency cleanup failed", "error", err)
							break
						}
						total += deleted
						if deleted < 1000 {
							break
						}
					}
					if total > 0 {
						slog.Info("idempotency expired keys cleaned", "rows", total)
					}
				}
				// Remember-me / trusted-device 过期行按批物理删除：读路径原本只在
				// 判定时过滤，过期行会随登录次数无限累积（OPEN-API-48）。
				now := time.Now().UTC()
				for _, cleanup := range []struct {
					name string
					fn   func(context.Context, time.Time, int) (int64, error)
				}{
					{"remember_me", authService.CleanupExpiredRememberMe},
					{"trusted_devices", authService.CleanupExpiredTrustedDevices},
				} {
					var total int64
					for i := 0; i < 10; i++ {
						deleted, err := cleanup.fn(ctx, now, 1000)
						if err != nil {
							slog.Warn("expired row cleanup failed", "table", cleanup.name, "error", err)
							break
						}
						total += deleted
						if deleted < 1000 {
							break
						}
					}
					if total > 0 {
						slog.Info("expired rows cleaned", "table", cleanup.name, "rows", total)
					}
				}
				cancel()
			case <-stopCh:
				slog.Info("session cleanup stopped")
				return
			}
		}
	})
	return stopCh
}
