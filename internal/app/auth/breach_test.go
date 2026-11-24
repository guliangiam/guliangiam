package auth

import (
	"context"
	"encoding/json"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubBreachChecker struct {
	breached bool
	err      error
	calls    int
}

func (s *stubBreachChecker) IsBreached(context.Context, string) (bool, error) {
	s.calls++
	return s.breached, s.err
}

func TestCheckPasswordBreach_NilCheckerPasses(t *testing.T) {
	s := &Service{}
	unverified, err := s.checkPasswordBreach(context.Background(), "any-password")
	require.NoError(t, err)
	assert.False(t, unverified, "开关关闭（nil）不得标记 unverified")
}

func TestCheckPasswordBreach_CleanPasses(t *testing.T) {
	stub := &stubBreachChecker{}
	s := &Service{BreachChecker: stub}
	unverified, err := s.checkPasswordBreach(context.Background(), "clean-password")
	require.NoError(t, err)
	assert.False(t, unverified)
	assert.Equal(t, 1, stub.calls)
}

func TestCheckPasswordBreach_BreachedIs422(t *testing.T) {
	s := &Service{BreachChecker: &stubBreachChecker{breached: true}}
	unverified, err := s.checkPasswordBreach(context.Background(), "pwned-password")
	require.Error(t, err)
	assert.False(t, unverified)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
}

// fail-open 裁定：依赖故障放行，unverified=true 供审计标记。
func TestCheckPasswordBreach_ErrorFailsOpenWithMarker(t *testing.T) {
	s := &Service{BreachChecker: &stubBreachChecker{err: apperrors.NewUnavailable("origin down")}}
	unverified, err := s.checkPasswordBreach(context.Background(), "any-password")
	require.NoError(t, err, "依赖故障必须 fail-open 放行")
	assert.True(t, unverified)
}

func TestWithBreachAuditMarker(t *testing.T) {
	payload := withBreachAuditMarker(map[string]any{"userId": "u1"}, true)
	assert.Equal(t, "unverified", payload["breachCheck"])
	assert.Equal(t, "u1", payload["userId"])

	payload = withBreachAuditMarker(map[string]any{}, false)
	assert.NotContains(t, payload, "breachCheck")
}

// flow 级：命中泄露库必须在写事务之前拒绝（无审计、凭据不变）。
func TestChangePassword_BreachedRejectedBeforeWrite(t *testing.T) {
	service, uow := newHistoryStack(0)
	service.BreachChecker = &stubBreachChecker{breached: true}
	hashBefore := uow.users.user.Credentials.PasswordHash

	_, _, err := service.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      testPassword,
		NewPassword:      "Br3ach!New2026",
	})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, 0, uow.records.created, "拒绝路径不得产生审计写入")
	assert.Equal(t, hashBefore, uow.users.user.Credentials.PasswordHash, "拒绝路径不得改凭据")
}

// flow 级：fail-open 放行且审计 outbox payload 带 breachCheck=unverified。
func TestChangePassword_BreachFailOpenMarksAudit(t *testing.T) {
	service, uow := newHistoryStack(0)
	service.BreachChecker = &stubBreachChecker{err: apperrors.NewUnavailable("origin down")}

	mustChangePassword(t, service, testPassword, "Br3ach!New2026")
	payload := changeAuditPayload(t, uow)
	assert.Equal(t, "unverified", payload["breachCheck"])
}

func TestChangePassword_BreachCleanNoMarker(t *testing.T) {
	service, uow := newHistoryStack(0)
	service.BreachChecker = &stubBreachChecker{}

	mustChangePassword(t, service, testPassword, "Br3ach!New2026")
	payload := changeAuditPayload(t, uow)
	assert.NotContains(t, payload, "breachCheck")
}

func changeAuditPayload(t *testing.T, uow *historyUoW) map[string]any {
	t.Helper()
	for _, event := range uow.outbox.events {
		if event.EventType != "auth.password.changed" {
			continue
		}
		var payload map[string]any
		require.NoError(t, json.Unmarshal(event.Payload, &payload))
		return payload
	}
	t.Fatal("no auth.password.changed outbox event found")
	return nil
}
