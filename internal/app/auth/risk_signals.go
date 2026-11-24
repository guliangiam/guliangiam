package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// collectRiskSignals 为内置风险引擎采集内部事实信号（BE-D-10）。
// 采集是尽力而为：单项失败只记日志并跳过该键，不阻断登录；风险门本身
// 仍 fail-closed。is_vpn/is_tor/geo_country 依赖外部 IP 情报源，无内部
// 生产者，登记为开放项 OPEN-API-77，不在这里伪造。
func (s Service) collectRiskSignals(ctx context.Context, tx appuow.UnitOfWork, orgID string, userID *string, deviceCredentialID, userAgent string, now time.Time) map[string]any {
	signals := map[string]any{}
	if userID == nil || strings.TrimSpace(*userID) == "" {
		return signals
	}
	uid := strings.TrimSpace(*userID)

	if repo := tx.LockoutAttempts(); repo != nil {
		attempt, err := repo.GetByUserID(ctx, uid)
		if err != nil {
			if !isRiskSignalNotFound(err) {
				slog.WarnContext(ctx, "risk signal collection: lockout lookup failed", "user_id", uid, "error", err)
			}
		} else if attempt != nil && attempt.Attempts > 0 &&
			!attempt.WindowStartedAt.IsZero() && now.Sub(attempt.WindowStartedAt) <= time.Hour {
			signals["recent_failures_1h"] = attempt.Attempts
		}
	}

	repo, err := trustedDeviceRepoFrom(tx)
	if err != nil || repo == nil {
		return signals
	}
	credentialID := strings.TrimSpace(deviceCredentialID)
	if credentialID == "" {
		// 本次登录未出示任何信任设备凭据 → 未知设备。
		signals["device_fingerprint_new"] = true
		return signals
	}
	digest, digestErr := trustedDeviceCredentialDigest(credentialID)
	if digestErr != nil {
		signals["device_fingerprint_new"] = true
		return signals
	}
	device, findErr := repo.FindActiveByCredential(ctx, orgID, uid, digest, trustedDeviceUAHash(userAgent), now)
	if findErr != nil {
		if isRiskSignalNotFound(findErr) {
			signals["device_fingerprint_new"] = true
		} else {
			slog.WarnContext(ctx, "risk signal collection: trusted device lookup failed", "user_id", uid, "error", findErr)
		}
		return signals
	}
	signals["device_fingerprint_new"] = device == nil || !device.Active(now)
	return signals
}

func isRiskSignalNotFound(err error) bool {
	var pe *apperrors.Error
	return errors.As(err, &pe) && pe.Code == apperrors.CodeNotFound
}
