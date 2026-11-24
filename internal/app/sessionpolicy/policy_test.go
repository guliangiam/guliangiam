package sessionpolicy

import (
	"context"
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
)

func TestIsIdleExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		now            time.Time
		lastActivityAt time.Time
		idleTimeout    time.Duration
		want           bool
	}{
		{"not expired", now, now.Add(-5 * time.Minute), 30 * time.Minute, false},
		{"exactly at timeout", now, now.Add(-30 * time.Minute), 30 * time.Minute, true},
		{"past timeout", now, now.Add(-60 * time.Minute), 30 * time.Minute, true},
		{"zero timeout", now, now.Add(-60 * time.Minute), 0, false},
		{"negative timeout", now, now.Add(-60 * time.Minute), -1, false},
		{"just before timeout", now, now.Add(-29*time.Minute - 59*time.Second), 30 * time.Minute, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsIdleExpired(tt.now, tt.lastActivityAt, tt.idleTimeout)
			if got != tt.want {
				t.Errorf("IsIdleExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldTouchActivity(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		now            time.Time
		lastActivityAt time.Time
		idleTimeout    time.Duration
		want           bool
	}{
		{"no timeout", now, now, 0, false},
		{"same time", now, now, 30 * time.Minute, false},
		{"within interval", now, now.Add(-2 * time.Minute), 30 * time.Minute, false},
		{"at interval", now, now.Add(-5 * time.Minute), 30 * time.Minute, true},
		{"past interval", now, now.Add(-10 * time.Minute), 30 * time.Minute, true},
		{"short timeout half interval", now, now.Add(-2 * time.Minute), 3 * time.Minute, true},
		{"very short timeout", now, now.Add(-2 * time.Minute), 1 * time.Minute, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldTouchActivity(tt.now, tt.lastActivityAt, tt.idleTimeout)
			if got != tt.want {
				t.Errorf("ShouldTouchActivity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNumericMinutes(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantVal int64
		wantOk  bool
	}{
		{"int", int(30), 30, true},
		{"int8", int8(15), 15, true},
		{"int16", int16(60), 60, true},
		{"int32", int32(45), 45, true},
		{"int64", int64(120), 120, true},
		{"uint", uint(30), 30, true},
		{"uint8", uint8(15), 15, true},
		{"uint16", uint16(60), 60, true},
		{"uint32", uint32(45), 45, true},
		{"uint64", uint64(120), 120, true},
		{"float32", float32(30.5), 30, true},
		{"float64", float64(30.7), 30, true},
		{"string", "30", 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := numericMinutes(tt.value)
			if val != tt.wantVal || ok != tt.wantOk {
				t.Errorf("numericMinutes() = (%d, %v), want (%d, %v)", val, ok, tt.wantVal, tt.wantOk)
			}
		})
	}
}

type mockOrgRepo struct {
	org *orgdomain.Organization
	err error
}

func (m *mockOrgRepo) GetByID(ctx context.Context, id string) (*orgdomain.Organization, error) {
	return m.org, m.err
}
func (m *mockOrgRepo) GetBySlug(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return m.org, m.err
}

// GetBySlugPublic delegates to GetBySlug (tests don't need RLS bypass).
func (m *mockOrgRepo) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return m.GetBySlug(ctx, slug)
}

func (m *mockOrgRepo) List(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	return nil, 0, m.err
}
func (m *mockOrgRepo) Create(ctx context.Context, org *orgdomain.Organization) error { return m.err }
func (m *mockOrgRepo) Update(ctx context.Context, org *orgdomain.Organization) error { return m.err }
func (m *mockOrgRepo) UpdateSettings(ctx context.Context, settings *orgdomain.OrganizationSettings) error {
	return m.err
}
func (m *mockOrgRepo) UpdateTheme(ctx context.Context, theme *orgdomain.OrganizationThemeSettings) error {
	return m.err
}
func (m *mockOrgRepo) UpdateSecurityPolicy(ctx context.Context, policy *orgdomain.OrganizationSecurityPolicy) error {
	return m.err
}
func (m *mockOrgRepo) Delete(ctx context.Context, id string) error { return m.err }
func (m *mockOrgRepo) HasDeletedWithSlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func TestResolveIdleTimeout(t *testing.T) {
	tests := []struct {
		name        string
		org         *orgdomain.Organization
		wantTimeout time.Duration
	}{
		{"with idle timeout", &orgdomain.Organization{
			Security: orgdomain.OrganizationSecurityPolicy{
				SessionPolicy: map[string]any{"idleTimeoutMinutes": 30},
			},
		}, 30 * time.Minute},
		{"zero timeout", &orgdomain.Organization{
			Security: orgdomain.OrganizationSecurityPolicy{
				SessionPolicy: map[string]any{"idleTimeoutMinutes": 0},
			},
		}, 0},
		{"negative timeout", &orgdomain.Organization{
			Security: orgdomain.OrganizationSecurityPolicy{
				SessionPolicy: map[string]any{"idleTimeoutMinutes": -5},
			},
		}, 0},
		{"missing key", &orgdomain.Organization{
			Security: orgdomain.OrganizationSecurityPolicy{
				SessionPolicy: map[string]any{},
			},
		}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockOrgRepo{org: tt.org}
			timeout, err := ResolveIdleTimeout(context.Background(), repo, "org-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if timeout != tt.wantTimeout {
				t.Errorf("ResolveIdleTimeout() = %v, want %v", timeout, tt.wantTimeout)
			}
		})
	}
}

func TestResolveIdleTimeout_EmptyOrgID(t *testing.T) {
	timeout, err := ResolveIdleTimeout(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if timeout != 0 {
		t.Errorf("expected 0 timeout for empty orgID, got %v", timeout)
	}
}

func TestClientMismatch(t *testing.T) {
	binding := ClientBinding{IP: true, UserAgent: true}
	tests := []struct {
		name      string
		sessionIP string
		sessionUA string
		requestIP string
		requestUA string
		binding   ClientBinding
		want      bool
	}{
		{"same client", "10.0.0.1", "Mozilla/5.0", "10.0.0.1", "Mozilla/5.0", binding, false},
		{"ip changed", "10.0.0.1", "Mozilla/5.0", "10.0.0.2", "Mozilla/5.0", binding, true},
		{"ua changed", "10.0.0.1", "Mozilla/5.0", "10.0.0.1", "curl/8", binding, true},
		{"binding disabled", "10.0.0.1", "Mozilla/5.0", "10.0.0.2", "curl/8", ClientBinding{}, false},
		{"empty request ip", "10.0.0.1", "Mozilla/5.0", "", "Mozilla/5.0", binding, false},
		{"empty session ua", "10.0.0.1", "", "10.0.0.1", "curl/8", binding, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClientMismatch(tt.sessionIP, tt.sessionUA, tt.requestIP, tt.requestUA, tt.binding); got != tt.want {
				t.Errorf("ClientMismatch() = %v, want %v", got, tt.want)
			}
		})
	}
}
