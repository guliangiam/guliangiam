package webhookevents

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

const (
	tenantOrgID    = "11111111-1111-1111-1111-111111111111"
	foreignWebhook = "33333333-3333-3333-3333-333333333333"
)

func TestRepository_List_QueryError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return nil, fmt.Errorf("db error")
		},
	}
	repo := NewRepository(mockDB)
	_, _, err := repo.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 10})
	require.Error(t, err)
}

func TestRepository_List_CountError(t *testing.T) {
	mockDB := &shared.MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			return shared.NewMockRows(nil), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, fmt.Errorf("count error"))
		},
	}
	repo := NewRepository(mockDB)
	_, _, err := repo.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 10})
	require.Error(t, err)
}

// The tenant predicate must be part of the SQL itself: webhook_events has no
// organization_id column (and no RLS), so ownership is resolved through the
// parent webhook. Bind it in the query so a cross-tenant parent simply yields
// no rows instead of being filtered after a full-table read.
func TestRepository_List_AppliesParentTenantPredicate(t *testing.T) {
	var listSQL, countSQL string
	var listArgs, countArgs []any

	mockDB := &shared.MockDBTX{
		QueryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			listSQL = sql
			listArgs = append([]any(nil), args...)
			return shared.NewMockRows(nil), nil
		},
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			countSQL = sql
			countArgs = append([]any(nil), args...)
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
	}

	items, total, err := NewRepository(mockDB).List(context.Background(), domain.ListQuery{
		Page: 1, PageSize: 10, WebhookID: foreignWebhook, OrganizationID: tenantOrgID,
	})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)

	const predicate = `EXISTS (SELECT 1 FROM "guliangiam".webhooks`
	for name, query := range map[string]string{"list": listSQL, "count": countSQL} {
		assert.Contains(t, query, predicate, "%s query must carry the parent tenant predicate", name)
		assert.Contains(t, query, "w.organization_id = $4::uuid", "%s query must bind the tenant", name)
	}

	// List and count share the same filter arguments, so the total cannot
	// disagree with the items (the pre-fix inconsistency risk).
	require.Len(t, listArgs, 6)
	require.Len(t, countArgs, 4)
	assert.Equal(t, tenantOrgID, listArgs[3])
	assert.Equal(t, tenantOrgID, countArgs[3])
	assert.Equal(t, listArgs[:4], countArgs)
	assert.Equal(t, 10, listArgs[4])
	assert.Equal(t, 0, listArgs[5])
}

// An empty tenant is the platform all-tenant scope (only reachable after the
// app-layer scope check) and must not be turned into a uuid predicate.
func TestRepository_List_EmptyTenantKeepsAllTenantScope(t *testing.T) {
	var listArgs, countArgs []any
	mockDB := &shared.MockDBTX{
		QueryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
			listArgs = append([]any(nil), args...)
			return shared.NewMockRows(nil), nil
		},
		QueryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			countArgs = append([]any(nil), args...)
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
	}

	_, _, err := NewRepository(mockDB).List(context.Background(), domain.ListQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, listArgs, 6)
	require.Len(t, countArgs, 4)
	assert.Equal(t, "", listArgs[3])
	assert.Equal(t, "", countArgs[3])
}

// ListExistingIDs must bind the canonical lowercase form (trimmed, blanks
// dropped) promised by the domain contract, so the fan-out can compare the
// returned set directly against deterministic delivery ids.
func TestRepository_ListExistingIDs_CanonicalizesAndDropsBlanks(t *testing.T) {
	var bound []string
	mockDB := &shared.MockDBTX{
		QueryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
			bound = append([]string(nil), args[0].([]string)...)
			return shared.NewMockRows([][]any{{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}), nil
		},
	}

	found, err := NewRepository(mockDB).ListExistingIDs(context.Background(), []string{
		" AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA ", "  ", "",
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
	}, bound, "ids canonicalized to lowercase and blanks dropped before binding")
	assert.Equal(t, []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}, found)
}

func TestRepository_ListExistingIDs_BlankOnlyShortCircuits(t *testing.T) {
	called := false
	mockDB := &shared.MockDBTX{
		QueryFn: func(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
			called = true
			return shared.NewMockRows(nil), nil
		},
	}
	found, err := NewRepository(mockDB).ListExistingIDs(context.Background(), []string{"", "  "})
	require.NoError(t, err)
	assert.Nil(t, found)
	assert.False(t, called, "blank-only input must not reach the database")
}

func TestQueryList_And_QueryCount_ShareTenantPredicate(t *testing.T) {
	snippet := `AND (CASE WHEN $4 = '' THEN TRUE ELSE EXISTS (SELECT 1 FROM "guliangiam".webhooks w WHERE w.id = webhook_events.webhook_id AND w.organization_id = $4::uuid) END)`
	assert.True(t, strings.Contains(QueryList, snippet), "QueryList missing parent tenant predicate: %s", QueryList)
	assert.True(t, strings.Contains(QueryCount, snippet), "QueryCount missing parent tenant predicate: %s", QueryCount)
}
