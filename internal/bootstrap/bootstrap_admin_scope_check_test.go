package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	platformadmindomain "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func fakeBootstrapAdmin(state bootstrapAdminState, found bool, err error) bootstrapAdminQuery {
	return func(context.Context) (bootstrapAdminState, bool, error) {
		return state, found, err
	}
}

func fakeScopeLister(
	active []*platformadmindomain.PlatformAdminScope,
	all []*platformadmindomain.PlatformAdminScope,
	err error,
	calls *int,
) bootstrapScopeLister {
	return func(_ context.Context, _ string, onlyActive bool) ([]*platformadmindomain.PlatformAdminScope, error) {
		if calls != nil {
			(*calls)++
		}
		if onlyActive {
			return active, err
		}
		return all, err
	}
}

func TestVerifyBootstrapAdminAllScope_SkipsWhenAdminAbsent(t *testing.T) {
	calls := 0
	err := verifyBootstrapAdminAllScope(
		context.Background(),
		fakeBootstrapAdmin(bootstrapAdminState{}, false, nil),
		fakeScopeLister(nil, nil, nil, &calls),
		true,
	)
	if err != nil {
		t.Fatalf("absent built-in admin must not fail startup: %v", err)
	}
	if calls != 0 {
		t.Fatalf("scope lister must not be called when admin is absent, calls=%d", calls)
	}
}

func TestVerifyBootstrapAdminAllScope_AcceptsValidAllScope(t *testing.T) {
	validAll := &platformadmindomain.PlatformAdminScope{
		ID:          "scope-1",
		ScopeType:   platformadmindomain.ScopeTypeAll,
		IsBootstrap: true,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	if err := verifyBootstrapAdminAllScope(
		context.Background(),
		fakeBootstrapAdmin(bootstrapAdminState{ID: "admin-1", IsActive: true}, true, nil),
		fakeScopeLister([]*platformadmindomain.PlatformAdminScope{validAll}, []*platformadmindomain.PlatformAdminScope{validAll}, nil, nil),
		true,
	); err != nil {
		t.Fatalf("valid all-scope must pass: %v", err)
	}
}

func TestVerifyBootstrapAdminAllScope_ReasonNoBootstrapRow(t *testing.T) {
	err := verifyBootstrapAdminAllScope(
		context.Background(),
		fakeBootstrapAdmin(bootstrapAdminState{ID: "admin-1", IsActive: true}, true, nil),
		fakeScopeLister(
			nil,
			[]*platformadmindomain.PlatformAdminScope{{ID: "scope-t", ScopeType: platformadmindomain.ScopeTypeTargeted}},
			nil, nil,
		),
		true,
	)
	assertUnavailableReason(t, err, bootstrapScopeReasonNoRow, "no all-tenant scope row")
}

func TestVerifyBootstrapAdminAllScope_ReasonExpired(t *testing.T) {
	expiredAt := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	expired := &platformadmindomain.PlatformAdminScope{
		ID:          "scope-1",
		ScopeType:   platformadmindomain.ScopeTypeAll,
		IsBootstrap: true,
		ExpiresAt:   expiredAt,
	}
	err := verifyBootstrapAdminAllScope(
		context.Background(),
		fakeBootstrapAdmin(bootstrapAdminState{ID: "admin-1", IsActive: true}, true, nil),
		fakeScopeLister(nil, []*platformadmindomain.PlatformAdminScope{expired}, nil, nil),
		true,
	)
	assertUnavailableReason(t, err, bootstrapScopeReasonExpired, "re-running -seed does NOT extend expires_at")
	if !strings.Contains(err.Error(), expiredAt.UTC().Format(time.RFC3339)) {
		t.Fatalf("error must include the expiry timestamp, got %v", err)
	}
}

func TestVerifyBootstrapAdminAllScope_ReasonFlagMissing(t *testing.T) {
	flagMissing := &platformadmindomain.PlatformAdminScope{
		ID:          "scope-1",
		ScopeType:   platformadmindomain.ScopeTypeAll,
		IsBootstrap: false,
		ApprovedBy:  nil,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	err := verifyBootstrapAdminAllScope(
		context.Background(),
		fakeBootstrapAdmin(bootstrapAdminState{ID: "admin-1", IsActive: true}, true, nil),
		fakeScopeLister(nil, []*platformadmindomain.PlatformAdminScope{flagMissing}, nil, nil),
		true,
	)
	assertUnavailableReason(t, err, bootstrapScopeReasonFlagMissing, "neither is_bootstrap nor approved_by")
}

func TestVerifyBootstrapAdminAllScope_ReasonInactive(t *testing.T) {
	validAll := &platformadmindomain.PlatformAdminScope{
		ID:          "scope-1",
		ScopeType:   platformadmindomain.ScopeTypeAll,
		IsBootstrap: true,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	err := verifyBootstrapAdminAllScope(
		context.Background(),
		fakeBootstrapAdmin(bootstrapAdminState{ID: "admin-1", IsActive: false}, true, nil),
		fakeScopeLister([]*platformadmindomain.PlatformAdminScope{validAll}, []*platformadmindomain.PlatformAdminScope{validAll}, nil, nil),
		true,
	)
	assertUnavailableReason(t, err, bootstrapScopeReasonInactive, "is_active=false")
}

func TestVerifyBootstrapAdminAllScope_WarnsButContinuesOutsideProduction(t *testing.T) {
	err := verifyBootstrapAdminAllScope(
		context.Background(),
		fakeBootstrapAdmin(bootstrapAdminState{ID: "admin-1", IsActive: true}, true, nil),
		fakeScopeLister(nil, nil, nil, nil),
		false,
	)
	if err != nil {
		t.Fatalf("non-production startup must continue with a warning: %v", err)
	}
}

func TestVerifyBootstrapAdminAllScope_PropagatesLookupErrors(t *testing.T) {
	err := verifyBootstrapAdminAllScope(
		context.Background(),
		fakeBootstrapAdmin(bootstrapAdminState{}, false, errors.New("query failed")),
		fakeScopeLister(nil, nil, nil, nil),
		false,
	)
	if err == nil {
		t.Fatal("lookup error must fail startup")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnavailable {
		t.Fatalf("error = %v, want unavailable", err)
	}
}

func TestVerifyBootstrapAdminAllScope_PropagatesScopeReadErrors(t *testing.T) {
	err := verifyBootstrapAdminAllScope(
		context.Background(),
		fakeBootstrapAdmin(bootstrapAdminState{ID: "admin-1", IsActive: true}, true, nil),
		fakeScopeLister(nil, nil, errors.New("read failed"), nil),
		false,
	)
	if err == nil {
		t.Fatal("scope read error must fail startup")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnavailable {
		t.Fatalf("error = %v, want unavailable", err)
	}
}

func assertUnavailableReason(t *testing.T, err error, reason, remediationFragment string) {
	t.Helper()
	if err == nil {
		t.Fatalf("production startup must fail with reason %q", reason)
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnavailable {
		t.Fatalf("error = %v, want unavailable", err)
	}
	if !strings.Contains(err.Error(), "reason="+reason) {
		t.Fatalf("error must name reason %q, got %v", reason, err)
	}
	if !strings.Contains(err.Error(), remediationFragment) {
		t.Fatalf("error must include remediation %q, got %v", remediationFragment, err)
	}
}

func TestExpiringBootstrapScope(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	scopes := []*platformadmindomain.PlatformAdminScope{
		{ID: "targeted", ScopeType: platformadmindomain.ScopeTypeTargeted, ExpiresAt: now.Add(24 * time.Hour)},
		{ID: "far", ScopeType: platformadmindomain.ScopeTypeAll, ExpiresAt: now.Add(90 * 24 * time.Hour)},
		{ID: "near", ScopeType: platformadmindomain.ScopeTypeAll, ExpiresAt: now.Add(10 * 24 * time.Hour)},
		{ID: "nearer", ScopeType: platformadmindomain.ScopeTypeAll, ExpiresAt: now.Add(3 * 24 * time.Hour)},
	}
	scopeID, expiresAt, remaining := expiringBootstrapScope(scopes, now)
	if scopeID != "nearer" {
		t.Fatalf("scopeID = %q, want nearer", scopeID)
	}
	if !expiresAt.Equal(now.Add(72 * time.Hour)) {
		t.Fatalf("expiresAt = %s", expiresAt)
	}
	if remaining != 72*time.Hour {
		t.Fatalf("remaining = %s", remaining)
	}

	if scopeID, _, _ := expiringBootstrapScope([]*platformadmindomain.PlatformAdminScope{
		{ID: "far", ScopeType: platformadmindomain.ScopeTypeAll, ExpiresAt: now.Add(90 * 24 * time.Hour)},
	}, now); scopeID != "" {
		t.Fatalf("far-future scope must not warn, got %q", scopeID)
	}
}
