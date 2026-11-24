package auth

import (
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

func TestEvaluatePasswordState(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-90 * 24 * time.Hour)

	tests := []struct {
		name       string
		credential userdomain.UserCredentials
		policy     orgdomain.PasswordPolicy
		wantMust   bool
		wantReason string
	}{
		{
			name:       "forced flag wins",
			credential: userdomain.UserCredentials{HasPassword: true, NeedUpdatePassword: true},
			policy:     orgdomain.PasswordPolicy{},
			wantMust:   true,
			wantReason: PasswordChangeReasonNeedUpdatePassword,
		},
		{
			name:       "no flag no max age",
			credential: userdomain.UserCredentials{HasPassword: true},
			policy:     orgdomain.PasswordPolicy{},
			wantMust:   false,
			wantReason: "",
		},
		{
			name: "max age exceeded",
			credential: userdomain.UserCredentials{
				HasPassword:       true,
				PasswordUpdatedAt: &updatedAt,
			},
			policy:     orgdomain.PasswordPolicy{PasswordMaxAgeDays: 60},
			wantMust:   true,
			wantReason: PasswordChangeReasonMaxAgeExceeded,
		},
		{
			name: "max age disabled",
			credential: userdomain.UserCredentials{
				HasPassword:       true,
				PasswordUpdatedAt: &updatedAt,
			},
			policy:     orgdomain.PasswordPolicy{PasswordMaxAgeDays: 0},
			wantMust:   false,
			wantReason: "",
		},
		{
			name:       "missing password update timestamp fails closed",
			credential: userdomain.UserCredentials{HasPassword: true},
			policy:     orgdomain.PasswordPolicy{PasswordMaxAgeDays: 60},
			wantMust:   true,
			wantReason: PasswordChangeReasonMaxAgeExceeded,
		},
		{
			name:       "no local password is never gated",
			credential: userdomain.UserCredentials{HasPassword: false, NeedUpdatePassword: true},
			policy:     orgdomain.PasswordPolicy{PasswordMaxAgeDays: 60},
			wantMust:   false,
			wantReason: "",
		},
		{
			name: "forced flag outranks max age",
			credential: userdomain.UserCredentials{
				HasPassword:        true,
				NeedUpdatePassword: true,
				PasswordUpdatedAt:  &updatedAt,
			},
			policy:     orgdomain.PasswordPolicy{PasswordMaxAgeDays: 60},
			wantMust:   true,
			wantReason: PasswordChangeReasonNeedUpdatePassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluatePasswordState(tt.credential, tt.policy, now)
			if got.MustChange != tt.wantMust {
				t.Fatalf("MustChange = %v, want %v", got.MustChange, tt.wantMust)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.wantReason)
			}
		})
	}
}
