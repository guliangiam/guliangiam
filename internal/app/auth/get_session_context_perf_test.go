package auth

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// perfSessionRepo records whether the authoritative session read was used and
// how many activity touches happened.
type perfSessionRepo struct {
	sessiondomain.Repository
	session      *sessiondomain.Session
	getByIDCalls int
	touchCalls   int
}

func (r *perfSessionRepo) GetByID(_ context.Context, id string) (*sessiondomain.Session, error) {
	r.getByIDCalls++
	if r.session != nil && r.session.ID == id {
		return r.session, nil
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *perfSessionRepo) Touch(_ context.Context, _ string, _ time.Time) error {
	r.touchCalls++
	return nil
}

// perfPolicyRepo implements the narrow SessionPolicyReader.
type perfPolicyRepo struct {
	orgdomain.Repository
	policy map[string]any
	calls  int
}

func (r *perfPolicyRepo) GetSessionPolicy(_ context.Context, _ string) (map[string]any, error) {
	r.calls++
	return r.policy, nil
}

// perfUOW is a UnitOfWork double that exposes the optional session-cache
// capability and counts transactions. The embedded nil interface supplies all
// repository accessors the read path does not use.
type perfUOW struct {
	appuow.UnitOfWork
	sessions sessiondomain.Repository
	orgs     orgdomain.Repository
	cached   *sessiondomain.Session
	txCount  int
}

func (u *perfUOW) Sessions() sessiondomain.Repository  { return u.sessions }
func (u *perfUOW) Organizations() orgdomain.Repository { return u.orgs }
func (u *perfUOW) Records() recordsdomain.Repository   { return nil }
func (u *perfUOW) CachedSession(_ context.Context, id string) (*sessiondomain.Session, bool) {
	if u.cached != nil && u.cached.ID == id {
		return u.cached, true
	}
	return nil, false
}

func (u *perfUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	u.txCount++
	return fn(ctx, u)
}

func perfActiveSession(now time.Time) *sessiondomain.Session {
	return &sessiondomain.Session{
		ID:               "sess-perf-1",
		UserID:           "user-perf-1",
		OrganizationID:   "org-perf-1",
		OrganizationSlug: "perf",
		SessionState:     sessiondomain.StateActive,
		ExpiresAt:        now.Add(time.Hour),
		LastSeenAt:       now,
		CreatedAt:        now,
	}
}

// TestResolveSessionContext_CacheHitUsesSinglePolicyReadTx pins the PERF-01
// win: a warm cache resolves the session with exactly one narrow, RLS-scoped
// policy-read transaction (organizations is FORCE RLS, so the read must run
// inside a transaction to get the tenant GUC) and never touches the sessions
// table.
func TestResolveSessionContext_CacheHitUsesSinglePolicyReadTx(t *testing.T) {
	now := time.Now().UTC()
	session := perfActiveSession(now)
	sessionRepo := &perfSessionRepo{}
	policyRepo := &perfPolicyRepo{policy: map[string]any{"idleTimeoutMinutes": 30}}
	uow := &perfUOW{sessions: sessionRepo, orgs: policyRepo, cached: session}

	svc := &Service{UOW: uow, Clock: fakeClock{fixedTime: now}}
	identity, err := svc.ResolveSessionContextForClient(context.Background(), session.ID, "", "")
	require.NoError(t, err)

	assert.Equal(t, 1, uow.txCount, "warm cache opens exactly the RLS-scoped policy read transaction")
	assert.Equal(t, 1, policyRepo.calls, "exactly one narrow policy read")
	assert.Equal(t, 0, sessionRepo.getByIDCalls, "sessions table must not be read")
	assert.Equal(t, "sess-perf-1", identity.ID)
}

// TestResolveSessionContext_CacheMissFallsBackToTransaction pins the fallback:
// without a cached row the authoritative transactional read still runs.
func TestResolveSessionContext_CacheMissFallsBackToTransaction(t *testing.T) {
	now := time.Now().UTC()
	session := perfActiveSession(now)
	sessionRepo := &perfSessionRepo{session: session}
	policyRepo := &perfPolicyRepo{policy: map[string]any{}}
	uow := &perfUOW{sessions: sessionRepo, orgs: policyRepo}

	svc := &Service{UOW: uow, Clock: fakeClock{fixedTime: now}}
	identity, err := svc.ResolveSessionContextForClient(context.Background(), session.ID, "", "")
	require.NoError(t, err)

	assert.Equal(t, 1, uow.txCount, "cache miss must keep the transactional read")
	assert.Equal(t, 1, sessionRepo.getByIDCalls)
	assert.Equal(t, 1, policyRepo.calls)
	assert.Equal(t, "sess-perf-1", identity.ID)
}

// TestResolveSessionContext_ActivityTouchUsesItsOwnTransaction pins the write
// decoupling: when the throttled window has elapsed, last_seen is written in a
// dedicated transaction instead of turning the authentication read into a
// write transaction.
func TestResolveSessionContext_ActivityTouchUsesItsOwnTransaction(t *testing.T) {
	now := time.Now().UTC()
	session := perfActiveSession(now)
	session.LastSeenAt = now.Add(-10 * time.Minute) // beyond the 5-minute window
	sessionRepo := &perfSessionRepo{}
	policyRepo := &perfPolicyRepo{policy: map[string]any{"idleTimeoutMinutes": 30}}
	uow := &perfUOW{sessions: sessionRepo, orgs: policyRepo, cached: session}

	svc := &Service{UOW: uow, Clock: fakeClock{fixedTime: now}}
	_, err := svc.ResolveSessionContextForClient(context.Background(), session.ID, "", "")
	require.NoError(t, err)

	assert.Equal(t, 2, uow.txCount, "policy-read transaction plus the dedicated touch transaction")
	assert.Equal(t, 1, sessionRepo.touchCalls)
	assert.Equal(t, 0, sessionRepo.getByIDCalls, "session row still comes from cache")
}
