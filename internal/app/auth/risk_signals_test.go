package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lockoutattempts "github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	riskdomain "github.com/guliangiam/guliangiam/internal/domain/risk"
	trusteddevicesdomain "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
)

// BE-D-10：内置引擎信号必须有内部事实生产者。采集失败只跳过键，不阻断登录。

func TestCollectRiskSignals_RecentFailuresWithinHour(t *testing.T) {
	now := time.Now()
	svc := Service{Clock: clock.SystemClock{}}
	uid := "user-1"
	uow := loginTestUnitOfWork{
		lockoutAttempt: &loginTestLockoutAttemptRepository{attempts: map[string]*lockoutattempts.Attempt{
			uid: {UserID: uid, Attempts: 4, WindowStartedAt: now.Add(-10 * time.Minute)},
		}},
	}
	signals := svc.collectRiskSignals(context.Background(), uow, "org-1", &uid, "", "ua", now)
	assert.Equal(t, 4, signals["recent_failures_1h"])

	uow.lockoutAttempt.attempts[uid].WindowStartedAt = now.Add(-2 * time.Hour)
	signals = svc.collectRiskSignals(context.Background(), uow, "org-1", &uid, "", "ua", now)
	_, present := signals["recent_failures_1h"]
	assert.False(t, present, "超出 1 小时窗口的失败计数不得计入")
}

func TestCollectRiskSignals_DeviceFingerprint(t *testing.T) {
	now := time.Now()
	svc := Service{Clock: clock.SystemClock{}}
	uid := "user-1"
	plaintext := strings.Repeat("a", 43)
	sum := sha256.Sum256([]byte(plaintext))
	digest := hex.EncodeToString(sum[:])
	repo := &capturingTrustedDeviceRepository{rows: []*trusteddevicesdomain.TrustedDevice{{
		ID: "dev-1", OrganizationID: "org-1", UserID: uid,
		DeviceCredentialID: digest, UAHash: trustedDeviceUAHash("ua"),
		CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour), UpdatedAt: now,
	}}}
	uow := loginTestUnitOfWork{trustedDeviceRepo: repo}

	signals := svc.collectRiskSignals(context.Background(), uow, "org-1", &uid, plaintext, "ua", now)
	assert.Equal(t, false, signals["device_fingerprint_new"], "命中有效信任设备 → 已知设备")
	assert.Zero(t, repo.touchedLastSeen, "信号采集不得产生 TouchLastSeen 副作用")

	signals = svc.collectRiskSignals(context.Background(), uow, "org-1", &uid, "", "ua", now)
	assert.Equal(t, true, signals["device_fingerprint_new"], "未出示凭据 → 未知设备")

	signals = svc.collectRiskSignals(context.Background(), uow, "org-1", &uid, strings.Repeat("b", 43), "ua", now)
	assert.Equal(t, true, signals["device_fingerprint_new"], "凭据不匹配 → 未知设备")

	signals = svc.collectRiskSignals(context.Background(), uow, "org-1", &uid, "short", "ua", now)
	assert.Equal(t, true, signals["device_fingerprint_new"], "非法凭据格式 → 未知设备")
}

func TestCollectRiskSignals_NoUserNoSignals(t *testing.T) {
	svc := Service{Clock: clock.SystemClock{}}
	signals := svc.collectRiskSignals(context.Background(), loginTestUnitOfWork{}, "org-1", nil, "", "ua", time.Now())
	assert.Empty(t, signals)
}

type capturingRiskEvaluator struct {
	gotSignals map[string]any
}

func (e *capturingRiskEvaluator) EvaluateAndRecordLoginRisk(_ context.Context, _ riskdomain.LoginRiskTx, cmd riskdomain.LoginRiskCommand) (riskdomain.LoginRiskResult, error) {
	e.gotSignals = cmd.Signals
	return riskdomain.LoginRiskResult{DecisionID: "dec-1", Decision: "allow"}, nil
}

func TestEvaluateLoginRisk_EnrichesAndCallerOverrides(t *testing.T) {
	now := time.Now()
	uid := "user-1"
	uow := loginTestUnitOfWork{
		lockoutAttempt: &loginTestLockoutAttemptRepository{attempts: map[string]*lockoutattempts.Attempt{
			uid: {UserID: uid, Attempts: 3, WindowStartedAt: now.Add(-5 * time.Minute)},
		}},
		trustedDeviceRepo: &capturingTrustedDeviceRepository{},
	}
	evaluator := &capturingRiskEvaluator{}
	svc := Service{Clock: clock.SystemClock{}, RiskEvaluator: evaluator}

	err := svc.evaluateLoginRisk(context.Background(), uow, "password", "org-1", &uid, "1.2.3.4", "ua", "", map[string]any{"custom": 1})
	require.NoError(t, err)
	require.NotNil(t, evaluator.gotSignals)
	assert.Equal(t, 3, evaluator.gotSignals["recent_failures_1h"])
	assert.Equal(t, true, evaluator.gotSignals["device_fingerprint_new"])
	assert.Equal(t, 1, evaluator.gotSignals["custom"])

	err = svc.evaluateLoginRisk(context.Background(), uow, "password", "org-1", &uid, "", "ua", "", map[string]any{"device_fingerprint_new": false})
	require.NoError(t, err)
	assert.Equal(t, false, evaluator.gotSignals["device_fingerprint_new"], "调用方显式键优先于采集值")
}
