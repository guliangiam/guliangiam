package privacy

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/privacy"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BE-D-22c：Grant 的 read-then-write 版本链必须在事务级 subject 锁内执行。

type fakeConsentRepo struct {
	latest      *domain.ConsentRecord
	latestErr   error
	lockErr     error
	createErr   error
	withdrawn   *domain.ConsentRecord
	withdrawErr error
	calls       []string
	created     []*domain.ConsentRecord
}

func (f *fakeConsentRepo) List(ctx context.Context, organizationID string, query domain.ConsentQuery) ([]*domain.ConsentRecord, error) {
	return nil, nil
}

func (f *fakeConsentRepo) LockSubject(ctx context.Context, organizationID, subjectType, subjectID, purpose string) error {
	f.calls = append(f.calls, "lock")
	return f.lockErr
}

func (f *fakeConsentRepo) Latest(ctx context.Context, organizationID, subjectType, subjectID, purpose string) (*domain.ConsentRecord, error) {
	f.calls = append(f.calls, "latest")
	if f.latestErr != nil {
		return nil, f.latestErr
	}
	if f.latest == nil {
		return nil, apperrors.NewNotFound("consent record not found")
	}
	return f.latest, nil
}

func (f *fakeConsentRepo) Create(ctx context.Context, record *domain.ConsentRecord) error {
	f.calls = append(f.calls, "create")
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, record)
	return nil
}

func (f *fakeConsentRepo) WithdrawLatest(ctx context.Context, organizationID, subjectType, subjectID, purpose string, at time.Time) (*domain.ConsentRecord, error) {
	f.calls = append(f.calls, "withdraw")
	if f.withdrawErr != nil {
		return nil, f.withdrawErr
	}
	if f.withdrawn == nil {
		return nil, apperrors.NewNotFound("no active consent to withdraw")
	}
	return f.withdrawn, nil
}

func newConsentService(repo *fakeConsentRepo, now time.Time) (ConsentService, *fakeTx) {
	tx := &fakeTx{consents: repo}
	return NewConsentService(fakeClock{now: now}, &fakeUUID{}, fakeRunner{tx: tx}), tx
}

func grantCommand() GrantConsentCommand {
	return GrantConsentCommand{
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		SubjectType:    domain.SubjectTypeUser,
		SubjectID:      "user-1",
		Purpose:        "marketing",
	}
}

func TestConsentGrant_FirstVersion(t *testing.T) {
	now := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	repo := &fakeConsentRepo{}
	svc, tx := newConsentService(repo, now)

	record, err := svc.Grant(context.Background(), grantCommand())
	require.NoError(t, err)
	assert.Equal(t, 1, record.Version)
	assert.Equal(t, []string{"lock", "latest", "create"}, repo.calls)
	require.Len(t, tx.audits, 1)
	assert.Equal(t, "privacy.consent.granted", tx.audits[0].Action)
	assert.Equal(t, "actor-1", tx.audits[0].ActorID)
}

func TestConsentGrant_IncrementsLatestVersion(t *testing.T) {
	now := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	repo := &fakeConsentRepo{latest: &domain.ConsentRecord{Version: 3}}
	svc, _ := newConsentService(repo, now)

	record, err := svc.Grant(context.Background(), grantCommand())
	require.NoError(t, err)
	assert.Equal(t, 4, record.Version)
	assert.Equal(t, []string{"lock", "latest", "create"}, repo.calls)
}

func TestConsentGrant_LockFailureAbortsBeforeRead(t *testing.T) {
	now := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	repo := &fakeConsentRepo{lockErr: apperrors.NewInternal("lock unavailable")}
	svc, tx := newConsentService(repo, now)

	_, err := svc.Grant(context.Background(), grantCommand())
	require.Error(t, err)
	assert.Equal(t, []string{"lock"}, repo.calls)
	assert.Empty(t, repo.created)
	assert.Empty(t, tx.audits)
}

func TestConsentGrant_LatestInfraErrorPropagates(t *testing.T) {
	now := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	repo := &fakeConsentRepo{latestErr: apperrors.NewInternal("db down")}
	svc, _ := newConsentService(repo, now)

	_, err := svc.Grant(context.Background(), grantCommand())
	require.Error(t, err)
	assert.Empty(t, repo.created)
}

func TestConsentGrant_ValidationBeforeTransaction(t *testing.T) {
	now := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	repo := &fakeConsentRepo{}
	svc, _ := newConsentService(repo, now)

	command := grantCommand()
	command.SubjectID = ""
	_, err := svc.Grant(context.Background(), command)
	require.Error(t, err)
	assert.Empty(t, repo.calls)
}

func TestConsentWithdraw_StampsAndAudits(t *testing.T) {
	now := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	withdrawnAt := now
	repo := &fakeConsentRepo{withdrawn: &domain.ConsentRecord{
		ID:          "consent-1",
		Version:     2,
		Purpose:     "marketing",
		WithdrawnAt: &withdrawnAt,
	}}
	svc, tx := newConsentService(repo, now)

	record, err := svc.Withdraw(context.Background(), WithdrawConsentCommand{
		OrganizationID: "org-1",
		ActorID:        "actor-1",
		SubjectID:      "user-1",
		Purpose:        "marketing",
	})
	require.NoError(t, err)
	assert.Equal(t, "consent-1", record.ID)
	require.Len(t, tx.audits, 1)
	assert.Equal(t, "privacy.consent.withdrawn", tx.audits[0].Action)
}

func TestConsentWithdraw_NoActiveConsentReturnsNotFound(t *testing.T) {
	now := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	repo := &fakeConsentRepo{}
	svc, tx := newConsentService(repo, now)

	_, err := svc.Withdraw(context.Background(), WithdrawConsentCommand{
		OrganizationID: "org-1",
		SubjectID:      "user-1",
		Purpose:        "marketing",
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	assert.Empty(t, tx.audits)
}
