package applications

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// TestRepository_ListActiveClientCertificates_TenantPredicate pins that the
// certificate read carries organization_id as an explicit predicate. A query
// issued for tenant B therefore cannot return tenant A rows.
func TestRepository_ListActiveClientCertificates_TenantPredicate(t *testing.T) {
	var gotSQL string
	var gotArgs []any
	mockDB := &shared.MockDBTX{
		QueryFn: func(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
			gotSQL = sql
			gotArgs = args
			return shared.NewMockRows(nil), nil
		},
	}

	certificates, err := NewRepository(mockDB).ListActiveClientCertificates(context.Background(), "org-B", "app-1")

	require.NoError(t, err)
	assert.Empty(t, certificates, "跨租户读取必须为空集")
	assert.Contains(t, gotSQL, "WHERE organization_id = $1 AND application_id = $2 AND status = 'active'")
	assert.Equal(t, []any{"org-B", "app-1"}, gotArgs)
}

// TestRepository_ReplaceClientCertificates_TenantPredicate pins both statements
// of the replace path: the retire UPDATE and the upsert INSERT each carry the
// tenant, and the upsert conflict target is scoped to (organization_id,
// fingerprint_sha256) so it can never move another tenant's row.
func TestRepository_ReplaceClientCertificates_TenantPredicate(t *testing.T) {
	replacedAt := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	var execSQL []string
	var execArgs [][]any
	var upsertSQL string
	var upsertArgs []any
	mockDB := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execSQL = append(execSQL, sql)
			execArgs = append(execArgs, args)
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		QueryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			upsertSQL = sql
			upsertArgs = args
			return shared.NewMockRow([]any{replacedAt, replacedAt, int64(1)}, nil)
		},
	}
	certificate := &domain.ClientCertificate{
		ID:                "cert-1",
		AuthMethod:        "tls_client_auth",
		CertificatePEM:    "-----BEGIN CERTIFICATE-----",
		FingerprintSHA256: "fp-1",
	}

	err := NewRepository(mockDB).ReplaceClientCertificates(context.Background(), "org-A", "app-1",
		[]*domain.ClientCertificate{certificate}, replacedAt)

	require.NoError(t, err)
	require.Len(t, execSQL, 1, "退役 active 行只允许一条语句")
	assert.Contains(t, execSQL[0], "WHERE organization_id = $1 AND application_id = $2 AND status = 'active'")
	assert.Equal(t, []any{"org-A", "app-1", replacedAt}, execArgs[0])
	assert.Contains(t, upsertSQL, "ON CONFLICT (organization_id, fingerprint_sha256)")
	assert.Equal(t, "org-A", upsertArgs[1], "写入行的 organization_id 来自调用方租户")
	assert.Equal(t, "app-1", upsertArgs[2])
	assert.Equal(t, "org-A", certificate.OrganizationID)
	assert.Equal(t, domain.ClientCertificateStatusActive, certificate.Status)
}
