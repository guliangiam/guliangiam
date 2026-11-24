package sessions

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/app/uow/mock"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	sessionmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupSessionTest(t *testing.T) (ctrlOut *gomock.Controller, uow *mock.MockUnitOfWork, sessions *sessionmock.MockRepository, orgs *orgmock.MockRepository, svcOut Service) {
	ctrl := gomock.NewController(t)
	uowMock := mock.NewMockUnitOfWork(ctrl)
	sessionRepo := sessionmock.NewMockRepository(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)
	svc := Service{
		Clock: clock.SystemClock{},
		UOW:   uowMock,
	}
	return ctrl, uowMock, sessionRepo, orgRepo, svc
}

func TestNewService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	uowMock := mock.NewMockUnitOfWork(ctrl)
	svc := NewService(clock.SystemClock{}, uowMock)
	assert.Equal(t, uowMock, svc.UOW)
}

func TestGetCurrentSession_Success(t *testing.T) {
	ctrl, uowMock, sessionRepo, orgRepo, svc := setupSessionTest(t)
	defer ctrl.Finish()

	now := time.Now().UTC()
	session := &domain.Session{
		ID:             "sess-1",
		OrganizationID: "org-1",
		ExpiresAt:      now.Add(1 * time.Hour),
		LastSeenAt:     now,
		SessionState:   domain.StateActive,
	}
	org := &orgdomain.Organization{
		ID: "org-1",
		Security: orgdomain.OrganizationSecurityPolicy{
			SessionPolicy: map[string]any{"idleTimeoutMinutes": 30},
		},
	}

	uowMock.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().GetByID(gomock.Any(), "sess-1").Return(session, nil)
	uowMock.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().GetByID(gomock.Any(), "org-1").Return(org, nil)

	result, err := svc.GetCurrentSession(context.Background(), "sess-1")
	require.NoError(t, err)
	assert.Equal(t, "sess-1", result.ID)
}

func TestGetCurrentSession_EmptyID(t *testing.T) {
	ctrl, _, _, _, svc := setupSessionTest(t)
	defer ctrl.Finish()

	_, err := svc.GetCurrentSession(context.Background(), "")
	assert.Error(t, err)
}

func TestGetCurrentSession_Revoked(t *testing.T) {
	ctrl, uowMock, sessionRepo, orgRepo, svc := setupSessionTest(t)
	defer ctrl.Finish()

	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)
	session := &domain.Session{
		ID:             "sess-1",
		OrganizationID: "org-1",
		ExpiresAt:      now.Add(1 * time.Hour),
		LastSeenAt:     now,
		RevokedAt:      &past,
	}
	org := &orgdomain.Organization{
		ID: "org-1",
		Security: orgdomain.OrganizationSecurityPolicy{
			SessionPolicy: map[string]any{"idleTimeoutMinutes": 30},
		},
	}

	uowMock.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().GetByID(gomock.Any(), "sess-1").Return(session, nil)
	uowMock.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().GetByID(gomock.Any(), "org-1").Return(org, nil)

	_, err := svc.GetCurrentSession(context.Background(), "sess-1")
	assert.Error(t, err)
}

func TestGetCurrentSession_Expired(t *testing.T) {
	ctrl, uowMock, sessionRepo, orgRepo, svc := setupSessionTest(t)
	defer ctrl.Finish()

	now := time.Now().UTC()
	session := &domain.Session{
		ID:             "sess-1",
		OrganizationID: "org-1",
		ExpiresAt:      now.Add(-1 * time.Hour), // expired
		LastSeenAt:     now.Add(-2 * time.Hour),
	}
	org := &orgdomain.Organization{
		ID: "org-1",
		Security: orgdomain.OrganizationSecurityPolicy{
			SessionPolicy: map[string]any{"idleTimeoutMinutes": 30},
		},
	}

	uowMock.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().GetByID(gomock.Any(), "sess-1").Return(session, nil)
	uowMock.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().GetByID(gomock.Any(), "org-1").Return(org, nil)

	_, err := svc.GetCurrentSession(context.Background(), "sess-1")
	assert.Error(t, err)
}

func TestUpdateSession_Success(t *testing.T) {
	ctrl, uowMock, sessionRepo, _, svc := setupSessionTest(t)
	defer ctrl.Finish()

	// § B2 写路径修复：UpdateSession 现在包在 WithinTransaction 里
	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uowMock)
		},
	)

	session := &domain.Session{
		ID:           "sess-1",
		SessionState: domain.StateActive,
		Revision:     1,
	}

	// WithinTransaction 里 Sessions() 调了 3 次 (GetByID + Update)
	uowMock.EXPECT().Sessions().Return(sessionRepo).Times(2)
	sessionRepo.EXPECT().GetByID(gomock.Any(), "sess-1").Return(session, nil)
	sessionRepo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

	newState := "revoked"
	err := svc.UpdateSession(context.Background(), UpdateSessionCommand{
		ID:           "sess-1",
		SessionState: &newState,
	})
	require.NoError(t, err)
}

func TestUpdateSession_EmptyID(t *testing.T) {
	ctrl, _, _, _, svc := setupSessionTest(t)
	defer ctrl.Finish()

	err := svc.UpdateSession(context.Background(), UpdateSessionCommand{
		ID: "",
	})
	assert.Error(t, err)
}

func TestUpdateSession_RevisionMismatch(t *testing.T) {
	ctrl, uowMock, sessionRepo, _, svc := setupSessionTest(t)
	defer ctrl.Finish()

	// § B2 写路径修复
	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uowMock)
		},
	)

	session := &domain.Session{
		ID:           "sess-1",
		SessionState: domain.StateActive,
		Revision:     2,
	}

	uowMock.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().GetByID(gomock.Any(), "sess-1").Return(session, nil)

	err := svc.UpdateSession(context.Background(), UpdateSessionCommand{
		ID:           "sess-1",
		SessionState: func() *string { s := "active"; return &s }(),
		Revision:     1,
	})
	assert.Error(t, err)
}

func TestUpdateSession_NoChanges(t *testing.T) {
	ctrl, uowMock, sessionRepo, _, svc := setupSessionTest(t)
	defer ctrl.Finish()

	// § B2 写路径修复
	uowMock.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
			return fn(ctx, uowMock)
		},
	)

	session := &domain.Session{
		ID:           "sess-1",
		SessionState: domain.StateActive,
		Revision:     1,
	}

	uowMock.EXPECT().Sessions().Return(sessionRepo)
	sessionRepo.EXPECT().GetByID(gomock.Any(), "sess-1").Return(session, nil)

	err := svc.UpdateSession(context.Background(), UpdateSessionCommand{
		ID:           "sess-1",
		SessionState: func() *string { s := "active"; return &s }(),
		Revision:     1,
	})
	require.NoError(t, err)
}
