package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// logoutTxTestUOW wires the records/outbox halves the logout transaction writes
// and hands the callback the outer struct, so the overrides are the ones the
// service sees (the embedded logoutTestUOW would pass the inner struct).
type logoutTxTestUOW struct {
	logoutTestUOW
	records       domainrecords.Repository
	outbox        domainoutbox.Repository
	apps          applicationdomain.Repository
	sessionsOuter sessiondomain.Repository
}

func (u logoutTxTestUOW) Records() domainrecords.Repository          { return u.records }
func (u logoutTxTestUOW) Outbox() domainoutbox.Repository            { return u.outbox }
func (u logoutTxTestUOW) Applications() applicationdomain.Repository { return u.apps }
func (u logoutTxTestUOW) Sessions() sessiondomain.Repository {
	if u.sessionsOuter != nil {
		return u.sessionsOuter
	}
	return u.logoutTestUOW.Sessions()
}
func (u logoutTxTestUOW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type logoutTxRecords struct {
	domainrecords.Repository
	created []*domainrecords.Record
	err     error
}

func (r *logoutTxRecords) Create(_ context.Context, record *domainrecords.Record) error {
	if r.err != nil {
		return r.err
	}
	r.created = append(r.created, record)
	return nil
}

type logoutTxOutbox struct {
	domainoutbox.Repository
	events        []*domainoutbox.OutboxEvent
	failEventType string
}

func (o *logoutTxOutbox) Enqueue(_ context.Context, item *domainoutbox.OutboxEvent) error {
	if o.failEventType != "" && item.EventType == o.failEventType {
		return apperrors.NewInternal("outbox unavailable")
	}
	o.events = append(o.events, item)
	return nil
}

func (o *logoutTxOutbox) EnqueueBatch(_ context.Context, items []*domainoutbox.OutboxEvent) error {
	o.events = append(o.events, items...)
	return nil
}

type logoutTxApps struct {
	applicationdomain.Repository
	apps []*applicationdomain.Application
}

func (a logoutTxApps) List(_ context.Context, _ applicationdomain.ListQuery) ([]*applicationdomain.Application, int64, error) {
	return a.apps, int64(len(a.apps)), nil
}

// deleteTrackingSessionRepo pins that SSO logout revokes rather than deletes.
type deleteTrackingSessionRepo struct {
	mockSessionRepoForLogout
	deleteCalled bool
}

func (r *deleteTrackingSessionRepo) Delete(ctx context.Context, id string) error {
	r.deleteCalled = true
	return r.mockSessionRepoForLogout.Delete(ctx, id)
}

func logoutTxSession() *sessiondomain.Session {
	return &sessiondomain.Session{
		ID:             "sess-001",
		UserID:         "user-001",
		OrganizationID: "org-1",
	}
}

func newLogoutTxService(repo *mockSessionRepoForLogout, records *logoutTxRecords, outbox *logoutTxOutbox) Service {
	return Service{
		Clock:     fakeClock{fixedTime: time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)},
		IssuerURL: "https://iam.example.com",
		UOW: logoutTxTestUOW{
			logoutTestUOW: logoutTestUOW{sessions: repo},
			records:       records,
			outbox:        outbox,
		},
	}
}

// TestLogout_CommitsRevokeAuditAndSSFTogether pins BE-06: the revoke, the audit
// record and the SSF revocation event are written through the same transaction
// handle, so none of them can commit without the others.
func TestLogout_CommitsRevokeAuditAndSSFTogether(t *testing.T) {
	repo := &mockSessionRepoForLogout{session: logoutTxSession()}
	records := &logoutTxRecords{}
	outbox := &logoutTxOutbox{}
	svc := newLogoutTxService(repo, records, outbox)

	require.NoError(t, svc.Logout(context.Background(), LogoutCommand{SessionID: "sess-001"}))
	assert.True(t, repo.revokeCalled)

	require.Len(t, records.created, 1)
	record := records.created[0]
	assert.Equal(t, "user.logout", record.Action)
	assert.Equal(t, "org-1", record.OrganizationID)
	assert.Equal(t, "user-001", record.ActorID)
	assert.Equal(t, "user", record.ResourceType)

	require.Len(t, outbox.events, 2)
	assert.Equal(t, "auth.logout", outbox.events[0].EventType)
	assert.Equal(t, "ssf.session.revoked", outbox.events[1].EventType)
	assert.Equal(t, "sess-001", outbox.events[1].AggregateID)
}

func TestLogout_RevokeFailureFailsWithoutAudit(t *testing.T) {
	repo := &mockSessionRepoForLogout{
		session:   logoutTxSession(),
		revokeErr: apperrors.NewInternal("database error"),
	}
	records := &logoutTxRecords{}
	outbox := &logoutTxOutbox{}
	svc := newLogoutTxService(repo, records, outbox)

	err := svc.Logout(context.Background(), LogoutCommand{SessionID: "sess-001"})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	assert.Empty(t, records.created, "a failed revoke must not leave an audit row")
	assert.Empty(t, outbox.events, "a failed revoke must not notify SSF receivers")
}

// TestLogout_SSFEnqueueFailureFailsLogout pins the atomicity direction: the
// revocation event shares the revoke's transaction, so an enqueue failure
// fails the logout instead of warning and reporting success.
func TestLogout_SSFEnqueueFailureFailsLogout(t *testing.T) {
	repo := &mockSessionRepoForLogout{session: logoutTxSession()}
	records := &logoutTxRecords{}
	outbox := &logoutTxOutbox{failEventType: "ssf.session.revoked"}
	svc := newLogoutTxService(repo, records, outbox)

	err := svc.Logout(context.Background(), LogoutCommand{SessionID: "sess-001"})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
}

func TestLogout_MissingSessionWritesNoAudit(t *testing.T) {
	repo := &mockSessionRepoForLogout{}
	records := &logoutTxRecords{}
	outbox := &logoutTxOutbox{}
	svc := newLogoutTxService(repo, records, outbox)

	require.NoError(t, svc.Logout(context.Background(), LogoutCommand{SessionID: "sess-missing"}))
	assert.True(t, repo.revokeCalled, "the idempotent revoke attempt is still made")
	assert.Empty(t, records.created)
	assert.Empty(t, outbox.events)
}

// TestSsoLogout_RevokesInsteadOfDeleting pins BE-07: the row is retained with
// revoked_at stamped, and the audit record commits with the revoke.
func TestSsoLogout_RevokesInsteadOfDeleting(t *testing.T) {
	repo := &deleteTrackingSessionRepo{
		mockSessionRepoForLogout: mockSessionRepoForLogout{session: logoutTxSession()},
	}
	records := &logoutTxRecords{}
	outbox := &logoutTxOutbox{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)},
		UOW: logoutTxTestUOW{
			logoutTestUOW: logoutTestUOW{sessions: &repo.mockSessionRepoForLogout},
			sessionsOuter: repo,
			records:       records,
			outbox:        outbox,
			apps: logoutTxApps{apps: []*applicationdomain.Application{
				{ID: "app-1", LogoutUrl: "https://app.example.com/logout"},
			}},
		},
	}

	result, err := svc.SsoLogout(context.Background(), "sess-001")
	require.NoError(t, err)
	assert.True(t, repo.revokeCalled)
	assert.False(t, repo.deleteCalled, "SSO logout must revoke, not physically delete")
	assert.Equal(t, []string{"https://app.example.com/logout"}, result.LogoutUrls)

	require.Len(t, records.created, 1)
	assert.Equal(t, "user.sso_logout", records.created[0].Action)
	require.Len(t, outbox.events, 1)
	assert.Equal(t, "auth.logout", outbox.events[0].EventType)
}

func TestSsoLogout_RevokeFailurePropagates(t *testing.T) {
	repo := &mockSessionRepoForLogout{
		session:   logoutTxSession(),
		revokeErr: apperrors.NewInternal("database error"),
	}
	records := &logoutTxRecords{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)},
		UOW: logoutTxTestUOW{
			logoutTestUOW: logoutTestUOW{sessions: repo},
			records:       records,
		},
	}

	_, err := svc.SsoLogout(context.Background(), "sess-001")
	require.Error(t, err, "a dependency failure must surface, not be logged and swallowed")
	assert.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	assert.Empty(t, records.created)
}

func TestSsoLogout_MissingSessionIsIdempotent(t *testing.T) {
	repo := &deleteTrackingSessionRepo{}
	records := &logoutTxRecords{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)},
		UOW: logoutTxTestUOW{
			logoutTestUOW: logoutTestUOW{sessions: &repo.mockSessionRepoForLogout},
			sessionsOuter: repo,
			records:       records,
		},
	}

	result, err := svc.SsoLogout(context.Background(), "sess-missing")
	require.NoError(t, err)
	assert.Empty(t, result.LogoutUrls)
	assert.False(t, repo.revokeCalled)
	assert.False(t, repo.deleteCalled)
	assert.Empty(t, records.created)
}

// TestCasLogout_AuditFailureFailsLogout pins BE-08: the audit write shares the
// revocations' transaction, so its failure fails the whole call (the real
// repository rolls the revocations back with it).
func TestCasLogout_AuditFailureFailsLogout(t *testing.T) {
	repo := &casLogoutSessionRepo{
		mockSessionRepoForLogout: casSessionFor(),
		fanOutRevoked:            2,
	}
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "rec-1"},
		UOW: casLogoutTestUOW{
			logoutTestUOW: logoutTestUOW{sessions: &repo.mockSessionRepoForLogout},
			sessions:      repo,
			apps:          logoutTxApps{apps: []*applicationdomain.Application{{ID: "app-1"}}},
			records:       &logoutTxRecords{err: apperrors.NewInternal("audit unavailable")},
		},
	}

	_, err := svc.CasLogout(context.Background(), CasLogoutCommand{SessionID: "sess-001"})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	assert.True(t, repo.fanOutCalled, "the failure comes from the audit write inside the same transaction")
}
