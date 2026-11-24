package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// OPEN-API-36 回归：MySQL 部署形态（pool=nil）下 dashboard / ldapsync 必须返回
// 受控的 503 unavailable，而不是 nil-pointer panic。
func TestUnavailableDashboardRepository_ReturnsUnavailable(t *testing.T) {
	repo := unavailableDashboardRepository{}

	_, err := repo.CountUsers(context.Background(), "org-1")
	assertUnavailable(t, err)

	for name, err := range map[string]error{
		"CountUsers":             err,
		"CountFailedLogins24h":   func() error { _, e := repo.CountFailedLogins24h(context.Background(), "org-1"); return e }(),
		"GetLoginHeatmap":        func() error { _, e := repo.GetLoginHeatmap(context.Background(), "org-1"); return e }(),
		"GetOrgUserDistribution": func() error { _, e := repo.GetOrgUserDistribution(context.Background()); return e }(),
	} {
		if err == nil {
			t.Fatalf("%s returned nil error", name)
		}
	}
}

func TestUnavailableLDAPSyncRepository_ReturnsUnavailable(t *testing.T) {
	repo := unavailableLDAPSyncRepository{}

	assertUnavailable(t, repo.CreateSyncConfig(context.Background(), nil))
	assertUnavailable(t, repo.MarkSyncRunning(context.Background(), "cfg-1"))

	if _, err := repo.ListDueSyncConfigs(context.Background()); err == nil {
		t.Fatal("ListDueSyncConfigs returned nil error")
	}
}

func assertUnavailable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an unavailable error, got nil")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeUnavailable {
		t.Fatalf("Code = %q, want %q", appErr.Code, apperrors.CodeUnavailable)
	}
	if status := appErr.HTTPStatus(); status != http.StatusServiceUnavailable {
		t.Fatalf("HTTPStatus = %d, want %d", status, http.StatusServiceUnavailable)
	}
}
