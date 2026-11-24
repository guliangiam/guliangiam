package b2b

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/b2b"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

type scanFunc func(...interface{}) error

func (f scanFunc) Scan(dest ...interface{}) error { return f(dest...) }

func TestScanOneAcceptsNullVerifiedAt(t *testing.T) {
	item, err := scanOne(scanFunc(func(dest ...interface{}) error {
		require.Len(t, dest, 12)
		require.IsType(t, (**time.Time)(nil), dest[5])
		require.IsType(t, (*int)(nil), dest[8])
		require.IsType(t, (**string)(nil), dest[9])
		require.IsType(t, (*time.Time)(nil), dest[10])
		require.IsType(t, (**time.Time)(nil), dest[11])
		return nil
	}))
	require.NoError(t, err)
	require.True(t, item.VerifiedAt.IsZero())
	require.Zero(t, item.LastError)
	require.True(t, item.LastCheckedAt.IsZero())
}

func TestCreateWritesNullForZeroVerifiedAt(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
		captured = args
		return pgconn.NewCommandTag("INSERT 1"), nil
	}}
	now := time.Now().UTC()
	err := NewRepository(db).Create(t.Context(), &domain.OrganizationDomain{
		ID: "domain-1", OrganizationID: "org-1", Domain: "corp.example.com",
		VerificationToken: "token", Status: domain.StatusPending,
		CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	require.Len(t, captured, 8)
	require.Nil(t, captured[5])
}

func TestFindVerifiedByDomainRejectsAmbiguousOwnership(t *testing.T) {
	row := func(id, orgID string) []any {
		return []any{
			id, orgID, "corp.example.com", "token", domain.StatusVerified, nil,
			time.Now(), time.Now(), 0, nil, time.Now(), nil,
		}
	}
	db := &shared.MockDBTX{QueryFn: func(_ context.Context, statement string, args ...any) (pgx.Rows, error) {
		require.Equal(t, QueryFindVerifiedByDomain, statement)
		require.Equal(t, []any{"corp.example.com"}, args)
		return shared.NewMockRows([][]any{row("domain-1", "org-1"), row("domain-2", "org-2")}), nil
	}}

	item, err := NewRepository(db).FindVerifiedByDomain(t.Context(), "corp.example.com")
	require.Nil(t, item)
	require.ErrorContains(t, err, "multiple verified organization domains")
}

func TestFindVerifiedByDomainReturnsSingleMatch(t *testing.T) {
	db := &shared.MockDBTX{QueryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return shared.NewMockRows([][]any{{
			"domain-1", "org-1", "corp.example.com", "token", domain.StatusVerified, nil,
			time.Now(), time.Now(), 0, nil, time.Now(), nil,
		}}), nil
	}}

	item, err := NewRepository(db).FindVerifiedByDomain(t.Context(), "corp.example.com")
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, "org-1", item.OrganizationID)
}

func TestUpdateRejectsNonPendingTransition(t *testing.T) {
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, statement string, _ ...any) (pgconn.CommandTag, error) {
		require.Equal(t, QueryUpdate, statement)
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}}
	err := NewRepository(db).Update(t.Context(), &domain.OrganizationDomain{
		ID: "domain-1", Domain: "corp.example.com", VerificationToken: "token",
		Status: domain.StatusVerified, UpdatedAt: time.Now(),
	})
	require.ErrorContains(t, err, "no longer pending")
}

func TestUpdateWritesRetryBudgetFields(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, statement string, args ...any) (pgconn.CommandTag, error) {
		require.Equal(t, QueryUpdate, statement)
		captured = args
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}}
	now := time.Now().UTC()
	nextRetry := now.Add(time.Minute)
	err := NewRepository(db).Update(t.Context(), &domain.OrganizationDomain{
		ID: "domain-1", Domain: "corp.example.com", VerificationToken: "token",
		Status: domain.StatusPending, Attempts: 1, LastError: "txt_missing: no TXT record found",
		NextRetryAt: nextRetry, LastCheckedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
	require.Len(t, captured, 10)
	require.Equal(t, 1, captured[5])
	require.Equal(t, "txt_missing: no TXT record found", captured[6])
	require.Equal(t, nextRetry, captured[7])
	require.Equal(t, now, captured[8])
	require.Equal(t, "domain-1", captured[9])
}

func TestScanOneReadsRetryBudgetFields(t *testing.T) {
	nextRetry := time.Now().UTC()
	checkedAt := nextRetry.Add(-time.Minute)
	item, err := scanOne(scanFunc(func(dest ...interface{}) error {
		ptr := dest[9].(**string)
		value := "dns_error: TXT lookup failed"
		*ptr = &value

		next := dest[10].(*time.Time)
		*next = nextRetry

		checked := dest[11].(**time.Time)
		*checked = &checkedAt

		attempts := dest[8].(*int)
		*attempts = 2
		return nil
	}))
	require.NoError(t, err)
	require.Equal(t, 2, item.Attempts)
	require.Equal(t, "dns_error: TXT lookup failed", item.LastError)
	require.Equal(t, nextRetry, item.NextRetryAt)
	require.Equal(t, checkedAt, item.LastCheckedAt)
}

func TestClaimPendingUsesAtomicClaimAndLease(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{QueryFn: func(_ context.Context, statement string, args ...any) (pgx.Rows, error) {
		require.Equal(t, QueryClaimPending, statement)
		captured = args
		return shared.NewMockRows([][]any{{
			"domain-1", "org-1", "corp.example.com", "token", domain.StatusPending, nil,
			time.Now(), time.Now(), 2, nil, time.Now().Add(5 * time.Minute), nil,
		}}), nil
	}}

	items, err := NewRepository(db).ClaimPending(t.Context(), 3, 300*time.Second)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, []any{3, "300 seconds"}, captured)
	require.Equal(t, 2, items[0].Attempts)
	require.Zero(t, items[0].LastError)
}

func TestClaimPendingQueryError(t *testing.T) {
	boom := errors.New("query failed")
	db := &shared.MockDBTX{QueryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
		return nil, boom
	}}
	_, err := NewRepository(db).ClaimPending(t.Context(), 1, time.Minute)
	require.ErrorIs(t, err, boom)
}

func TestLeaseIntervalClampsToAtLeastOneSecond(t *testing.T) {
	require.Equal(t, "300 seconds", leaseInterval(5*time.Minute))
	require.Equal(t, "1 seconds", leaseInterval(0))
	require.Equal(t, "1 seconds", leaseInterval(500*time.Millisecond))
}

func TestGetByIDForOrganization_TenantPredicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, statement string, args ...any) pgx.Row {
		gotSQL = statement
		gotArgs = args
		return shared.NewMockRow(nil, pgx.ErrNoRows)
	}}

	_, err := NewRepository(db).GetByIDForOrganization(t.Context(), "domain-1", "org-1")
	var notFound *apperrors.Error
	require.ErrorAs(t, err, &notFound)
	require.Equal(t, apperrors.CodeNotFound, notFound.Code)
	require.Contains(t, gotSQL, "id = $1::uuid")
	require.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"domain-1", "org-1"}, gotArgs)
}

func TestDelete_TenantPredicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, statement string, args ...any) (pgconn.CommandTag, error) {
		gotSQL = statement
		gotArgs = args
		return pgconn.NewCommandTag("DELETE 0"), nil
	}}

	err := NewRepository(db).Delete(t.Context(), "domain-1", "org-1")
	var notFound *apperrors.Error
	require.ErrorAs(t, err, &notFound)
	require.Equal(t, apperrors.CodeNotFound, notFound.Code)
	require.Contains(t, gotSQL, "organization_id = guliangiam.resolve_org_id($2)")
	require.Equal(t, []any{"domain-1", "org-1"}, gotArgs)
}
