package subscriptions

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestSubscription_Validate(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)
	further := now.Add(48 * time.Hour)
	tests := []struct {
		name    string
		sub     Subscription
		wantErr bool
		errCode apperrors.Code
	}{
		{
			name: "valid active subscription",
			sub: Subscription{
				Name: "pro-monthly", State: "active",
			},
			wantErr: false,
		},
		{
			name: "valid pending subscription",
			sub: Subscription{
				Name: "trial", State: "pending",
			},
			wantErr: false,
		},
		{
			name: "valid trialing subscription",
			sub: Subscription{
				Name: "trial", State: "trialing",
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			sub: Subscription{
				Name: "bad", State: "unknown",
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "valid date range",
			sub: Subscription{
				Name: "sub-1", State: "active",
				StartAt:  &now,
				EndAt:    &future,
				ExpireAt: &further,
			},
			wantErr: false,
		},
		{
			name: "endAt before startAt",
			sub: Subscription{
				Name: "sub-2", State: "active",
				StartAt: &future,
				EndAt:   &now,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "expireAt before endAt",
			sub: Subscription{
				Name: "sub-3", State: "active",
				StartAt:  &now,
				EndAt:    &further,
				ExpireAt: &future,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "expireAt before startAt",
			sub: Subscription{
				Name: "sub-4", State: "active",
				StartAt:  &future,
				ExpireAt: &now,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "nil dates valid",
			sub: Subscription{
				Name: "sub-5", State: "active",
				StartAt: nil, EndAt: nil, ExpireAt: nil,
			},
			wantErr: false,
		},
		{
			name: "cancelled subscription",
			sub: Subscription{
				Name: "sub-6", State: "cancelled",
			},
			wantErr: false,
		},
		{
			name: "expired subscription",
			sub: Subscription{
				Name: "sub-7", State: "expired",
			},
			wantErr: false,
		},
		{
			name: "past_due subscription",
			sub: Subscription{
				Name: "sub-8", State: "past_due",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sub.Validate()
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

func TestSubscription_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"pending -> active", "pending", "active", true},
		{"pending -> cancelled", "pending", "cancelled", true},
		{"pending -> expired", "pending", "expired", false},
		{"trialing -> active", "trialing", "active", true},
		{"trialing -> cancelled", "trialing", "cancelled", true},
		{"trialing -> past_due", "trialing", "past_due", true},
		{"active -> cancelled", "active", "cancelled", true},
		{"active -> expired", "active", "expired", true},
		{"active -> past_due", "active", "past_due", true},
		{"active -> pending", "active", "pending", false},
		{"past_due -> active", "past_due", "active", true},
		{"past_due -> cancelled", "past_due", "cancelled", true},
		{"past_due -> expired", "past_due", "expired", true},
		{"cancelled -> active", "cancelled", "active", false},
		{"cancelled -> pending", "cancelled", "pending", false},
		{"expired -> active", "expired", "active", false},
		{"unknown -> active", "unknown", "active", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Subscription{State: tt.from}
			got := s.CanTransitionTo(tt.to)
			if got != tt.want {
				t.Errorf("CanTransitionTo(%q -> %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
