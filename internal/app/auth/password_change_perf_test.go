package auth

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// perfPasswordUsersRepo records, for each user-repository call, whether a
// transaction was open. This is the observable form of "CPU-bound password
// verification moved out of the transaction".
type perfPasswordUsersRepo struct {
	*historyUserRepo
	uow              *perfPasswordUOW
	flipOnSecondRead bool
}

func (r *perfPasswordUsersRepo) GetByUsername(ctx context.Context, organizationID, username string) (*userdomain.User, error) {
	r.uow.getByUsernameInTx = append(r.uow.getByUsernameInTx, r.uow.inTx)
	user, err := r.historyUserRepo.GetByUsername(ctx, organizationID, username)
	if err != nil {
		return nil, err
	}
	if r.flipOnSecondRead && len(r.uow.getByUsernameInTx) > 1 {
		clone := *user
		clone.Credentials.PasswordHash = "concurrently-changed-hash"
		return &clone, nil
	}
	return user, nil
}

func (r *perfPasswordUsersRepo) GetPasswordHistory(ctx context.Context, userID string, limit int) ([]userdomain.PasswordHistoryEntry, error) {
	r.uow.historyReadInTx = append(r.uow.historyReadInTx, r.uow.inTx)
	return r.historyUserRepo.GetPasswordHistory(ctx, userID, limit)
}

func (r *perfPasswordUsersRepo) UpdatePasswordCredential(ctx context.Context, credentials *userdomain.UserCredentials) error {
	r.uow.updateInTx = append(r.uow.updateInTx, r.uow.inTx)
	return r.historyUserRepo.UpdatePasswordCredential(ctx, credentials)
}

type perfPasswordUOW struct {
	*historyUoW
	users             userdomain.Repository
	inTx              bool
	getByUsernameInTx []bool
	historyReadInTx   []bool
	updateInTx        []bool
}

func (u *perfPasswordUOW) Users() userdomain.Repository { return u.users }

func (u *perfPasswordUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	u.inTx = true
	defer func() { u.inTx = false }()
	return fn(ctx, u)
}

func newPerfPasswordService(t *testing.T, flipOnSecondRead bool) (*Service, *perfPasswordUOW) {
	t.Helper()
	_, baseUOW := newHistoryStack(5)
	users := &perfPasswordUsersRepo{historyUserRepo: baseUOW.users, flipOnSecondRead: flipOnSecondRead}
	uow := &perfPasswordUOW{historyUoW: baseUOW, users: users}
	users.uow = uow

	service := &Service{
		UOW:           uow,
		Clock:         fakeClock{fixedTime: time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "generated-id"},
	}
	return service, uow
}

// TestChangePassword_CPUBoundChecksRunOutsideTransaction pins PERF-04: reads
// run in one short RLS-scoped transaction (organizations/users/password
// histories are FORCE RLS tables and need the tenant GUC), while the old
// password verification and the history reuse comparison (argon2 over every
// fetched row) run outside any transaction; the write transaction only
// performs the revalidation read plus writes.
func TestChangePassword_CPUBoundChecksRunOutsideTransaction(t *testing.T) {
	service, uow := newPerfPasswordService(t, false)

	_, _, err := service.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      testPassword,
		NewPassword:      "Hist0ry!B2026",
	})
	require.NoError(t, err)

	require.Len(t, uow.historyReadInTx, 1, "history is read exactly once")
	assert.True(t, uow.historyReadInTx[0], "history read must run inside the RLS-scoped read transaction")

	assert.Equal(t, []bool{true, true}, uow.getByUsernameInTx,
		"first read (verification snapshot) inside the read tx, second read (revalidation) inside the write tx")
	assert.Equal(t, []bool{true}, uow.updateInTx, "credential write stays inside the transaction")
}

// TestChangePassword_ConcurrentChangeRejected pins the semantic guard added by
// moving verification out: if the stored hash changed between the verified
// snapshot and the write transaction, nothing is written.
func TestChangePassword_ConcurrentChangeRejected(t *testing.T) {
	service, uow := newPerfPasswordService(t, true)

	_, _, err := service.ChangePassword(context.Background(), ChangePasswordCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		OldPassword:      testPassword,
		NewPassword:      "Hist0ry!B2026",
	})
	require.Error(t, err)

	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)

	assert.Empty(t, uow.updateInTx, "no credential write after a concurrent change")
	assert.Zero(t, uow.records.created, "no audit record after a concurrent change")
	assert.Zero(t, uow.outbox.enqueued, "no outbox event after a concurrent change")
}
