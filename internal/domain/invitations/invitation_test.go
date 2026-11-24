package invitations

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestInvitation_IsExpired(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		inv         Invitation
		now         time.Time
		wantExpired bool
	}{
		{
			name:        "not expired - future expiresAt",
			inv:         Invitation{ExpiresAt: now.Add(time.Hour)},
			now:         now,
			wantExpired: false,
		},
		{
			name:        "expired - past expiresAt",
			inv:         Invitation{ExpiresAt: now.Add(-time.Hour)},
			now:         now,
			wantExpired: true,
		},
		{
			name:        "not expired - zero expiresAt",
			inv:         Invitation{ExpiresAt: time.Time{}},
			now:         now,
			wantExpired: false,
		},
		{
			name:        "not expired - exactly at expiresAt",
			inv:         Invitation{ExpiresAt: now},
			now:         now,
			wantExpired: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inv.IsExpired(tt.now)
			if got != tt.wantExpired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.wantExpired)
			}
		})
	}
}

func TestInvitation_IsAcceptable(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		inv            Invitation
		now            time.Time
		wantAcceptable bool
	}{
		{
			name:           "pending and not expired",
			inv:            Invitation{Status: StatusPending, ExpiresAt: now.Add(time.Hour)},
			now:            now,
			wantAcceptable: true,
		},
		{
			name:           "pending but expired",
			inv:            Invitation{Status: StatusPending, ExpiresAt: now.Add(-time.Hour)},
			now:            now,
			wantAcceptable: false,
		},
		{
			name:           "accepted status",
			inv:            Invitation{Status: StatusAccepted, ExpiresAt: now.Add(time.Hour)},
			now:            now,
			wantAcceptable: false,
		},
		{
			name:           "revoked status",
			inv:            Invitation{Status: StatusRevoked, ExpiresAt: now.Add(time.Hour)},
			now:            now,
			wantAcceptable: false,
		},
		{
			name:           "expired status",
			inv:            Invitation{Status: StatusExpired, ExpiresAt: now.Add(time.Hour)},
			now:            now,
			wantAcceptable: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inv.IsAcceptable(tt.now)
			if got != tt.wantAcceptable {
				t.Errorf("IsAcceptable() = %v, want %v", got, tt.wantAcceptable)
			}
		})
	}
}

func TestInvitation_Validate(t *testing.T) {
	now := time.Now().Add(24 * time.Hour)
	tests := []struct {
		name    string
		inv     Invitation
		wantErr bool
		errCode apperrors.Code
	}{
		{
			name: "valid invitation",
			inv: Invitation{
				Email:     "user@example.com",
				Code:      "abc123",
				Status:    StatusPending,
				ExpiresAt: now,
			},
			wantErr: false,
		},
		{
			name: "missing email",
			inv: Invitation{
				Code:      "abc123",
				Status:    StatusPending,
				ExpiresAt: now,
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "missing code",
			inv: Invitation{
				Email:     "user@example.com",
				Status:    StatusPending,
				ExpiresAt: now,
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "invalid status",
			inv: Invitation{
				Email:     "user@example.com",
				Code:      "abc123",
				Status:    "unknown",
				ExpiresAt: now,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "missing expiresAt",
			inv: Invitation{
				Email:  "user@example.com",
				Code:   "abc123",
				Status: StatusPending,
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "valid accepted status",
			inv: Invitation{
				Email:     "user@example.com",
				Code:      "abc123",
				Status:    StatusAccepted,
				ExpiresAt: now,
			},
			wantErr: false,
		},
		{
			name: "valid revoked status",
			inv: Invitation{
				Email:     "user@example.com",
				Code:      "abc123",
				Status:    StatusRevoked,
				ExpiresAt: now,
			},
			wantErr: false,
		},
		{
			name: "valid expired status",
			inv: Invitation{
				Email:     "user@example.com",
				Code:      "abc123",
				Status:    StatusExpired,
				ExpiresAt: now,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.inv.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				appErr, ok := err.(*apperrors.Error)
				if !ok {
					t.Fatalf("error type = %T, want *apperrors.Error", err)
				}
				if appErr.Code != tt.errCode {
					t.Errorf("error code = %q, want %q", appErr.Code, tt.errCode)
				}
			}
		})
	}
}

func TestInvitation_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name       string
		fromStatus string
		toStatus   string
		want       bool
	}{
		{"pending -> accepted", StatusPending, StatusAccepted, true},
		{"pending -> revoked", StatusPending, StatusRevoked, true},
		{"pending -> expired", StatusPending, StatusExpired, true},
		{"pending -> pending", StatusPending, StatusPending, false},
		{"accepted -> pending", StatusAccepted, StatusPending, false},
		{"accepted -> revoked", StatusAccepted, StatusRevoked, false},
		{"revoked -> pending", StatusRevoked, StatusPending, false},
		{"expired -> pending", StatusExpired, StatusPending, false},
		{"unknown -> pending", "unknown", StatusPending, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := Invitation{Status: tt.fromStatus}
			got := inv.CanTransitionTo(tt.toStatus)
			if got != tt.want {
				t.Errorf("CanTransitionTo(%q -> %q) = %v, want %v", tt.fromStatus, tt.toStatus, got, tt.want)
			}
		})
	}
}
