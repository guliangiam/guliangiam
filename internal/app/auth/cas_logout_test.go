package auth

import (
	"context"
	"testing"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	applicationmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	idvdomain "github.com/guliangiam/guliangiam/internal/domain/idv"
	igadomain "github.com/guliangiam/guliangiam/internal/domain/iga"
	rebac "github.com/guliangiam/guliangiam/internal/domain/rebac"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	recordsmock "github.com/guliangiam/guliangiam/internal/domain/records/mock"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	workloaddomain "github.com/guliangiam/guliangiam/internal/domain/workload"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// casLogoutSessionRepo records what the SLO fan-out was asked to do.
type casLogoutSessionRepo struct {
	mockSessionRepoForLogout
	fanOutCalled     bool
	fannedOutUserID  string
	fanOutExcluded   string
	fanOutRevoked    int64
	fanOutErr        error
	fanOutRevokedAt  time.Time
	revokeReceivedAt time.Time
}

func (r *casLogoutSessionRepo) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	r.revokeReceivedAt = revokedAt
	return r.mockSessionRepoForLogout.Revoke(ctx, id, revokedAt)
}

func (r *casLogoutSessionRepo) RevokeByUserExcept(ctx context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	r.fanOutCalled = true
	r.fannedOutUserID = userID
	r.fanOutExcluded = excludeSessionID
	r.fanOutRevokedAt = revokedAt
	return r.fanOutRevoked, r.fanOutErr
}

func (r *casLogoutSessionRepo) RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *casLogoutSessionRepo) RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

// casLogoutAuditSink captures the audit record the service wrote, so a test can
// assert on the record itself instead of on a mock call count.
type casLogoutAuditSink struct {
	record *recordsdomain.Record
}

func (s *casLogoutAuditSink) Write(_ context.Context, record *recordsdomain.Record) error {
	s.record = record
	return nil
}

type casLogoutTestUOW struct {
	logoutTestUOW
	sessions *casLogoutSessionRepo
	apps     applicationdomain.Repository
	records  recordsdomain.Repository
}

func (u casLogoutTestUOW) Outbox() domainoutbox.Repository { return nil }
func (u casLogoutTestUOW) IDV() idvdomain.Repository       { return nil }

// The embedded logoutTestUOW.WithinTransaction would hand the callback the
// inner struct, bypassing the Sessions/Records overrides above.
func (u casLogoutTestUOW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u casLogoutTestUOW) Workload() workloaddomain.Repository { return nil }

func (u casLogoutTestUOW) Rebac() rebac.Repository { return nil }

func (u casLogoutTestUOW) Sessions() sessiondomain.Repository { return u.sessions }

func (u casLogoutTestUOW) Applications() applicationdomain.Repository { return u.apps }

func (u casLogoutTestUOW) Records() recordsdomain.Repository { return u.records }
func (u casLogoutTestUOW) IGA() igadomain.Repository         { return nil }

// newCasLogoutService wires a Service with a redirect-URI whitelist and an audit
// sink, so each test can assert one requirement at a time.
func newCasLogoutService(
	t *testing.T,
	repo *casLogoutSessionRepo,
	redirectURIs []string,
) (Service, *casLogoutAuditSink) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	apps := applicationmock.NewMockRepository(ctrl)
	apps.EXPECT().
		List(gomock.Any(), gomock.Any()).
		Return([]*applicationdomain.Application{{ID: "app-1", RedirectURIs: redirectURIs}}, int64(1), nil).
		AnyTimes()

	sink := &casLogoutAuditSink{}
	records := recordsmock.NewMockRepository(ctrl)
	records.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, record *recordsdomain.Record) error {
			return sink.Write(context.Background(), record)
		}).
		AnyTimes()

	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "rec-1"},
		UOW: casLogoutTestUOW{
			logoutTestUOW: logoutTestUOW{sessions: &repo.mockSessionRepoForLogout},
			sessions:      repo,
			apps:          apps,
			records:       records,
		},
	}
	return svc, sink
}

// casSessionFor is the subject of every SLO case: one session, one identity,
// one organization. No case varies the session itself (they vary how the
// repository answers and what the caller asks for), so the fixture takes no
// arguments — a parameter every caller passes the same value to reads as
// configurability the suite does not have.
func casSessionFor() mockSessionRepoForLogout {
	return mockSessionRepoForLogout{
		session: &sessiondomain.Session{
			ID:             "sess-001",
			UserID:         "user-001",
			OrganizationID: "org-1",
			ApplicationID:  "app-1",
		},
	}
}

func TestCasLogout_RevokesSubjectAcrossApplications(t *testing.T) {
	repo := &casLogoutSessionRepo{
		mockSessionRepoForLogout: casSessionFor(),
		fanOutRevoked:            3,
	}
	svc, _ := newCasLogoutService(t, repo, nil)

	result, err := svc.CasLogout(context.Background(), CasLogoutCommand{SessionID: "sess-001"})
	require.NoError(t, err)

	assert.Equal(t, 4, result.RevokedSessions, "the current session plus the fan-out")
	assert.True(t, repo.revokeCalled)
	assert.True(t, repo.fanOutCalled)
	assert.Equal(t, "user-001", repo.fannedOutUserID)
	assert.Equal(t, "sess-001", repo.fanOutExcluded, "the current session must not be counted twice")
	assert.Equal(t, repo.revokeReceivedAt, repo.fanOutRevokedAt, "one clock reading covers the whole logout")
}

func TestCasLogout_WritesAuditRecord(t *testing.T) {
	repo := &casLogoutSessionRepo{
		mockSessionRepoForLogout: casSessionFor(),
		fanOutRevoked:            2,
	}
	svc, sink := newCasLogoutService(t, repo, nil)

	_, err := svc.CasLogout(context.Background(), CasLogoutCommand{
		SessionID: "sess-001",
		IPAddress: "203.0.113.7",
		UserAgent: "Mozilla/5.0",
		Surface:   "protocol",
		RequestID: "req-1",
		TraceID:   "trace-1",
	})
	require.NoError(t, err)
	require.NotNil(t, sink.record)

	assert.Equal(t, CasLogoutAction, sink.record.Action)
	assert.Equal(t, "org-1", sink.record.OrganizationID)
	assert.Equal(t, "user-001", sink.record.ActorID)
	assert.Equal(t, "user", sink.record.ResourceType)
	assert.Equal(t, "203.0.113.7", sink.record.IP)
	assert.Equal(t, "Mozilla/5.0", sink.record.UserAgent)
	assert.Equal(t, recordsdomain.SurfaceProtocol, sink.record.Surface)
	assert.Equal(t, "req-1", sink.record.RequestID)
	assert.Equal(t, "trace-1", sink.record.TraceID)
	assert.Equal(t, 3, sink.record.Metadata["revokedSessions"])
	assert.Equal(t, false, sink.record.Metadata["redirectRejected"])
}

func TestCasLogout_RegisteredServiceIsEchoed(t *testing.T) {
	repo := &casLogoutSessionRepo{mockSessionRepoForLogout: casSessionFor()}
	svc, _ := newCasLogoutService(t, repo, []string{"https://app.example.com/cb"})

	result, err := svc.CasLogout(context.Background(), CasLogoutCommand{
		SessionID: "sess-001",
		Service:   "https://app.example.com/cb",
	})
	require.NoError(t, err)

	assert.Equal(t, "https://app.example.com/cb", result.RedirectURI)
	assert.False(t, result.RedirectRejected)
}

func TestCasLogout_UnregisteredServiceIsRefused(t *testing.T) {
	repo := &casLogoutSessionRepo{mockSessionRepoForLogout: casSessionFor()}
	svc, _ := newCasLogoutService(t, repo, []string{"https://app.example.com/cb"})

	result, err := svc.CasLogout(context.Background(), CasLogoutCommand{
		SessionID: "sess-001",
		Service:   "https://evil.example.com/steal",
	})
	require.NoError(t, err)

	assert.Empty(t, result.RedirectURI)
	assert.True(t, result.RedirectRejected)
	assert.True(t, repo.revokeCalled, "a refused redirect must not cancel the logout")
}

// TestCasLogout_RefusesRedirectWhenNoRedirectURIRegistered pins the empty-set
// rule of applications.MatchRedirectURI: an application that registered nothing
// admits nothing.
func TestCasLogout_RefusesRedirectWhenNoRedirectURIRegistered(t *testing.T) {
	repo := &casLogoutSessionRepo{mockSessionRepoForLogout: casSessionFor()}
	svc, _ := newCasLogoutService(t, repo, nil)

	result, err := svc.CasLogout(context.Background(), CasLogoutCommand{
		SessionID: "sess-001",
		Service:   "https://app.example.com/cb",
	})
	require.NoError(t, err)

	assert.Empty(t, result.RedirectURI)
	assert.True(t, result.RedirectRejected)
}

func TestCasLogout_UnknownSessionIsIdempotent(t *testing.T) {
	repo := &casLogoutSessionRepo{}
	svc, sink := newCasLogoutService(t, repo, nil)

	result, err := svc.CasLogout(context.Background(), CasLogoutCommand{SessionID: "sess-missing"})
	require.NoError(t, err)

	assert.Equal(t, 0, result.RevokedSessions)
	assert.False(t, result.RedirectRejected)
	assert.False(t, repo.revokeCalled)
	assert.False(t, repo.fanOutCalled)
	assert.Nil(t, sink.record, "a logout that changed nothing writes no audit row")
}

func TestCasLogout_EmptySessionIDIsNoOp(t *testing.T) {
	repo := &casLogoutSessionRepo{}
	svc, _ := newCasLogoutService(t, repo, []string{"https://app.example.com/cb"})

	result, err := svc.CasLogout(context.Background(), CasLogoutCommand{Service: "https://app.example.com/cb"})
	require.NoError(t, err)

	assert.Equal(t, 0, result.RevokedSessions)
	assert.Empty(t, result.RedirectURI)
	assert.False(t, repo.revokeCalled)
}

func TestCasLogout_RevokedSessionIsIdempotent(t *testing.T) {
	// A session whose row is already revoked is reported as not-found by the
	// repository; the repeat visit must still look like a success.
	repo := &casLogoutSessionRepo{
		mockSessionRepoForLogout: mockSessionRepoForLogout{
			revokeErr: apperrors.NewNotFound("session not found"),
		},
	}
	svc, _ := newCasLogoutService(t, repo, nil)

	result, err := svc.CasLogout(context.Background(), CasLogoutCommand{SessionID: "sess-001"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.RevokedSessions)
}

func TestCasLogout_PropagatesFanOutFailure(t *testing.T) {
	repo := &casLogoutSessionRepo{
		mockSessionRepoForLogout: casSessionFor(),
		fanOutErr:                apperrors.NewInternal("fan-out failed"),
	}
	svc, _ := newCasLogoutService(t, repo, nil)

	_, err := svc.CasLogout(context.Background(), CasLogoutCommand{SessionID: "sess-001"})
	require.Error(t, err)

	typed, ok := err.(*apperrors.Error)
	require.True(t, ok, "a fan-out failure must surface as a typed application error")
	assert.Equal(t, apperrors.CodeInternal, typed.Code)
}
