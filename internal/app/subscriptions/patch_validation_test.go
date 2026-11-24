package subscriptions

import (
	"context"
	"testing"
	"time"

	subscriptiondomain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// TestPatchSubscription_RevalidatesTimeWindow pins BE-C-14: a PATCH whose
// merged time window violates start < end < expire must be rejected before
// the write, exactly like the create path.
func TestPatchSubscription_RevalidatesTimeWindow(t *testing.T) {
	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now.Add(24 * time.Hour)
	beforeStart := start.Add(-time.Hour)
	tests := []struct {
		name    string
		cmd     PatchSubscriptionCommand
		wantErr bool
	}{
		{name: "endAt before startAt rejected", cmd: PatchSubscriptionCommand{ID: "x-1", EndAt: &beforeStart, Revision: 1}, wantErr: true},
		{name: "valid window accepted", cmd: PatchSubscriptionCommand{ID: "x-1", EndAt: &end, Revision: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockSubscriptionRepo{subscriptions: []*subscriptiondomain.Subscription{{
				ID: "x-1", OrganizationID: "org-1", Name: "test", State: subscriptiondomain.StateActive,
				StartAt: &start, Revision: 1, CreatedAt: now, UpdatedAt: now,
			}}}
			svc := newSvc(repo)
			err := svc.PatchSubscription(context.Background(), "org-1", tt.cmd)
			if tt.wantErr {
				if err == nil {
					t.Fatal("PatchSubscription() error = nil, want time-window validation rejection")
				}
				if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
					t.Fatalf("error code = %s, want %s", code, apperrors.CodeUnprocessableEntity)
				}
				if repo.subscriptions[0].Revision != 1 {
					t.Errorf("Revision = %d, want 1 (no write on rejection)", repo.subscriptions[0].Revision)
				}
				return
			}
			if err != nil {
				t.Fatalf("PatchSubscription() error = %v", err)
			}
		})
	}
}
