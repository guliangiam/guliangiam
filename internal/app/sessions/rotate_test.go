package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
)

// RotateCurrentSession 只把「会话已不存在」当作可以静默跳过的幂等场景；
// 其他存储错误必须上抛，避免数据库故障时静默不轮换会话（安全缺陷）。
func TestService_RotateCurrentSession(t *testing.T) {
	now := time.Now()
	storageErr := errors.New("database unavailable")

	tests := []struct {
		name       string
		sessionID  string
		repo       *mockSessionRepo
		wantNewID  string
		wantErr    error
		wantRevoke bool
	}{
		{
			name:      "empty session id is a no-op",
			sessionID: "",
			repo:      &mockSessionRepo{},
			wantNewID: "",
		},
		{
			name:      "missing session is a no-op",
			sessionID: "missing",
			repo:      &mockSessionRepo{},
			wantNewID: "",
		},
		{
			name:      "storage failure is propagated",
			sessionID: "sess-1",
			repo:      &mockSessionRepo{getErr: storageErr},
			wantErr:   storageErr,
		},
		{
			name:      "existing session is rotated",
			sessionID: "sess-1",
			repo: &mockSessionRepo{
				sessions: []*domain.Session{
					{ID: "sess-1", UserID: "user-1", OrganizationID: "org-1"},
				},
			},
			wantRevoke: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{
				sessionRepo: tt.repo,
				recordsRepo: &mockRecordsRepo{},
				outboxRepo:  &mockOutboxRepo{},
			}}

			newID, err := svc.RotateCurrentSession(context.Background(), tt.sessionID)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("RotateCurrentSession() error = %v, want %v", err, tt.wantErr)
				}
				if newID != "" {
					t.Errorf("RotateCurrentSession() newID = %q, want empty on failure", newID)
				}
				return
			}
			if err != nil {
				t.Fatalf("RotateCurrentSession() unexpected error = %v", err)
			}
			if tt.wantRevoke {
				if newID == "" {
					t.Error("expected a new session id")
				}
				if tt.repo.sessions[0].RevokedAt == nil {
					t.Error("expected old session to be revoked")
				}
				return
			}
			if newID != "" {
				t.Errorf("RotateCurrentSession() newID = %q, want empty", newID)
			}
		})
	}
}
