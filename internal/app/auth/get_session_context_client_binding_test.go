package auth

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	recordsmock "github.com/guliangiam/guliangiam/internal/domain/records/mock"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type bindingTestUOW struct {
	logoutTestUOW
	orgs orgdomain.Repository
}

func (u bindingTestUOW) Organizations() orgdomain.Repository { return u.orgs }

// WithinTransaction must pass this wrapper (not the embedded logoutTestUOW),
// otherwise tx.Organizations() inside the service would still be nil.
func (u bindingTestUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func bindingSession() *sessiondomain.Session {
	now := time.Now().UTC()
	return &sessiondomain.Session{
		ID:             "sess-bind-1",
		UserID:         "user-1",
		OrganizationID: "org-1",
		SessionState:   sessiondomain.StateActive,
		IPAddress:      "10.0.0.1",
		UserAgent:      "Mozilla/5.0",
		ExpiresAt:      now.Add(time.Hour),
		LastSeenAt:     now,
		CreatedAt:      now,
	}
}

func bindingService(t *testing.T, orgs orgdomain.Repository) *Service {
	t.Helper()
	return &Service{
		UOW:   bindingTestUOW{logoutTestUOW{sessions: &mockSessionRepoForLogout{session: bindingSession()}}, orgs},
		Clock: fakeClock{fixedTime: time.Now().UTC()},
	}
}

func TestResolveSessionContextForClient_BindingEnabledRejectsChangedIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	orgs := orgmock.NewMockRepository(ctrl)
	orgs.EXPECT().GetByID(gomock.Any(), "org-1").Return(&orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{SessionPolicy: map[string]any{"bindIp": true}},
	}, nil)

	_, err := bindingService(t, orgs).ResolveSessionContextForClient(context.Background(), "sess-bind-1", "10.0.0.2", "Mozilla/5.0")
	require.Error(t, err)
}

func TestResolveSessionContextForClient_BindingEnabledRejectsChangedUserAgent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	orgs := orgmock.NewMockRepository(ctrl)
	orgs.EXPECT().GetByID(gomock.Any(), "org-1").Return(&orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{SessionPolicy: map[string]any{"bindUserAgent": true}},
	}, nil)

	_, err := bindingService(t, orgs).ResolveSessionContextForClient(context.Background(), "sess-bind-1", "10.0.0.1", "curl/8")
	require.Error(t, err)
}

func TestResolveSessionContextForClient_BindingDisabledAllowsChangedClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	orgs := orgmock.NewMockRepository(ctrl)
	orgs.EXPECT().GetByID(gomock.Any(), "org-1").Return(&orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{SessionPolicy: map[string]any{}},
	}, nil)

	identity, err := bindingService(t, orgs).ResolveSessionContextForClient(context.Background(), "sess-bind-1", "10.0.0.2", "curl/8")
	require.NoError(t, err)
	assert.Equal(t, "sess-bind-1", identity.ID)
	assert.Equal(t, "user-1", identity.UserID)
}

type recordsBindingUOW struct {
	bindingTestUOW
	records recordsdomain.Repository
}

func (u recordsBindingUOW) Records() recordsdomain.Repository { return u.records }

func TestResolveSessionContextForClient_WritesMismatchAudit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	orgs := orgmock.NewMockRepository(ctrl)
	orgs.EXPECT().GetByID(gomock.Any(), "org-1").Return(&orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{SessionPolicy: map[string]any{"bindIp": true}},
	}, nil)
	recordRepo := recordsmock.NewMockRepository(ctrl)
	recordRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, record *recordsdomain.Record) error {
			assert.Equal(t, "auth.session.client_mismatch", record.Action)
			assert.Equal(t, "sess-bind-1", record.ResourceID)
			assert.Equal(t, "user-1", record.ActorID)
			return nil
		})
	base := bindingTestUOW{
		logoutTestUOW: logoutTestUOW{sessions: &mockSessionRepoForLogout{session: bindingSession()}},
		orgs:          orgs,
	}
	svc := &Service{
		UOW:   recordsBindingUOW{bindingTestUOW: base, records: recordRepo},
		Clock: fakeClock{fixedTime: time.Now().UTC()},
	}
	_, err := svc.ResolveSessionContextForClient(context.Background(), "sess-bind-1", "10.0.0.2", "Mozilla/5.0")
	require.Error(t, err)
}
