package main

import (
	"context"
	"database/sql/driver"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
	"github.com/guliangiam/guliangiam/internal/platform/pagination"
)

func sealedPayloadColumnsForTable(t *testing.T, table string) []crypto.SealedPayloadColumn {
	t.Helper()
	for _, col := range crypto.SealedPayloadColumns {
		if col.Table == table {
			return []crypto.SealedPayloadColumn{col}
		}
	}
	t.Fatalf("sealed payload table %s is not registered", table)
	return nil
}

func expectSealedPayloadScan(mock sqlmock.Sqlmock, col crypto.SealedPayloadColumn, batchSize int, rows *sqlmock.Rows) {
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT %s, %s FROM %s ORDER BY %s ASC LIMIT $1",
		col.PKColumn, col.Column, col.Table, col.PKColumn))).
		WithArgs(batchSize).
		WillReturnRows(rows)
}

func sealedPayloadRows(col crypto.SealedPayloadColumn, values ...driver.Value) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{col.PKColumn, col.Column})
	for i := 0; i+1 < len(values); i += 2 {
		rows.AddRow(values[i], values[i+1])
	}
	return rows
}

func expectSealedPayloadUpdate(mock sqlmock.Sqlmock, col crypto.SealedPayloadColumn, encryptor *crypto.FieldEncryptor, pk, plaintext string) {
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
		"UPDATE %s SET %s = $1 WHERE %s = $2 AND %s = $3", col.Table, col.Column, col.PKColumn, col.Column))).
		WithArgs(sealedPayloadV1Arg{encryptor: encryptor, plaintext: plaintext}, pk, plaintext).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

// The registry is the domain selection: one run must scan both
// llm_providers.api_key_sealed and pay_channel_configs.config_sealed.
func TestEncryptSealedPayloads_TableDrivenCoversPayChannel(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	llm, pay := crypto.SealedPayloadColumns[0], crypto.SealedPayloadColumns[1]

	expectSealedPayloadScan(mock, llm, 10, sealedPayloadRows(llm, "p1", "sk-plain"))
	expectSealedPayloadUpdate(mock, llm, encryptor, "p1", "sk-plain")
	expectSealedPayloadScan(mock, pay, 10, sealedPayloadRows(pay, "c1", `{"apiKey":"plain"}`))
	expectSealedPayloadUpdate(mock, pay, encryptor, "c1", `{"apiKey":"plain"}`)

	var code int
	out := captureStdout(t, func() {
		code = encryptSealedPayloadsWithDB(context.Background(), sqlDB, encryptor, crypto.SealedPayloadColumns,
			crypto.SealedPayloadBackfillOptions{BatchSize: 10})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "guliangiam.llm_providers.api_key_sealed: scanned=1 selected=1 encrypted=1")
	assert.Contains(t, out, "guliangiam.pay_channel_configs.config_sealed: scanned=1 selected=1 encrypted=1")
	assert.Contains(t, out, "summary: scanned=2 selected=2 encrypted=2")
	assert.NotContains(t, out, "sk-plain", "the summary must never print plaintext")
	assert.NotContains(t, out, `{"apiKey":"plain"}`)
	t.Log(out)
	require.NoError(t, mock.ExpectationsWereMet())
}

// -table narrows the registry-driven run without hardcoding a domain.
func TestEncryptSealedPayloads_TableFilterSelectsPayChannel(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	pay := crypto.SealedPayloadColumns[1]
	expectSealedPayloadScan(mock, pay, 10, sealedPayloadRows(pay, "c1", `{"apiKey":"plain"}`))
	expectSealedPayloadUpdate(mock, pay, encryptor, "c1", `{"apiKey":"plain"}`)

	var code int
	out := captureStdout(t, func() {
		code = encryptSealedPayloadsWithDB(context.Background(), sqlDB, encryptor,
			sealedPayloadColumnsForTable(t, pay.Table), crypto.SealedPayloadBackfillOptions{BatchSize: 10})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "guliangiam.pay_channel_configs.config_sealed: scanned=1 selected=1 encrypted=1")
	assert.NotContains(t, out, "guliangiam.llm_providers")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEncryptSealedPayloads_SealsPlaintextAndSkipsEnvelopeAndEmpty(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	col := crypto.SealedPayloadColumns[1]
	v1, err := encryptor.EncryptField("already-sealed")
	require.NoError(t, err)

	expectSealedPayloadScan(mock, col, 10, sealedPayloadRows(col,
		"c1", `{"apiKey":"legacy-plain"}`,
		"c2", v1,
		"c3", "",
		"c4", nil))
	expectSealedPayloadUpdate(mock, col, encryptor, "c1", `{"apiKey":"legacy-plain"}`)

	var code int
	out := captureStdout(t, func() {
		code = encryptSealedPayloadsWithDB(context.Background(), sqlDB, encryptor, []crypto.SealedPayloadColumn{col},
			crypto.SealedPayloadBackfillOptions{BatchSize: 10})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "scanned=4 selected=1 encrypted=1 resealed=0 skipped=3 failed=0")
	assert.NotContains(t, out, "legacy-plain", "the summary must never print plaintext")
	assert.NotContains(t, out, v1, "the summary must never print ciphertext")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEncryptSealedPayloads_SecondRunIsNoop(t *testing.T) {
	col := crypto.SealedPayloadColumns[1]
	plaintext := `{"apiKey":"legacy-a"}`

	firstDB, firstMock, firstEncryptor := newBackfillDB(t)
	expectSealedPayloadScan(firstMock, col, 10, sealedPayloadRows(col, "c1", plaintext))
	expectSealedPayloadUpdate(firstMock, col, firstEncryptor, "c1", plaintext)
	out := captureStdout(t, func() {
		assert.Equal(t, 0, encryptSealedPayloadsWithDB(context.Background(), firstDB, firstEncryptor,
			[]crypto.SealedPayloadColumn{col}, crypto.SealedPayloadBackfillOptions{BatchSize: 10}))
	})
	assert.Contains(t, out, "selected=1 encrypted=1")
	require.NoError(t, firstMock.ExpectationsWereMet())

	sealed, err := firstEncryptor.EncryptField(plaintext)
	require.NoError(t, err)

	secondDB, secondMock, secondEncryptor := newBackfillDB(t)
	expectSealedPayloadScan(secondMock, col, 10, sealedPayloadRows(col, "c1", sealed))
	captureStdout(t, func() {
		assert.Equal(t, 0, encryptSealedPayloadsWithDB(context.Background(), secondDB, secondEncryptor,
			[]crypto.SealedPayloadColumn{col}, crypto.SealedPayloadBackfillOptions{BatchSize: 10}))
	})
	require.NoError(t, secondMock.ExpectationsWereMet(), "a re-run over sealed rows issues no UPDATE")
}

func TestEncryptSealedPayloads_VerifyReportedAsAADMismatchAndPlaintext(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	col := crypto.SealedPayloadColumns[1]
	v2, err := encryptor.EncryptColumn(col.AAD("c2"), "sealed-config")
	require.NoError(t, err)
	moved, err := encryptor.EncryptColumn(col.AAD("c9"), "moved-config")
	require.NoError(t, err)
	expectSealedPayloadScan(mock, col, 10, sealedPayloadRows(col,
		"c1", "plaintext-config",
		"c2", v2,
		"c3", moved))

	var code int
	out := captureStdout(t, func() {
		code = encryptSealedPayloadsWithDB(context.Background(), sqlDB, encryptor, []crypto.SealedPayloadColumn{col},
			crypto.SealedPayloadBackfillOptions{BatchSize: 10, Verify: true})
	})

	assert.Equal(t, 1, code)
	assert.Contains(t, out, "plaintext_remaining=1")
	assert.Contains(t, out, "aad_mismatch=1")
	assert.Contains(t, out, "FAIL guliangiam.pay_channel_configs.config_sealed pk=c1 class=unrecognized stage=verify reason=plaintext_remaining")
	assert.Contains(t, out, "FAIL guliangiam.pay_channel_configs.config_sealed pk=c3 class=unreadable stage=verify reason=aad_mismatch")
	assert.NotContains(t, out, "plaintext-config")
	assert.NotContains(t, out, moved)
	t.Log(out)
	require.NoError(t, mock.ExpectationsWereMet(), "verify must issue no UPDATE")
}

func TestEncryptSealedPayloads_VerifyAcceptsRegisteredAAD(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	col := crypto.SealedPayloadColumns[1]
	v1, err := encryptor.EncryptField("v1-config")
	require.NoError(t, err)
	v2, err := encryptor.EncryptColumn(col.AAD("c2"), "v2-config")
	require.NoError(t, err)
	expectSealedPayloadScan(mock, col, 10, sealedPayloadRows(col, "c1", v1, "c2", v2, "c3", ""))

	var code int
	out := captureStdout(t, func() {
		code = encryptSealedPayloadsWithDB(context.Background(), sqlDB, encryptor, []crypto.SealedPayloadColumn{col},
			crypto.SealedPayloadBackfillOptions{BatchSize: 10, Verify: true})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "plaintext_remaining=0")
	assert.Contains(t, out, "aad_mismatch=0")
	assert.Contains(t, out, "verify OK")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEncryptSealedPayloads_DryRunListsWouldSealWithoutWriting(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)
	col := crypto.SealedPayloadColumns[1]
	v1, err := encryptor.EncryptField("already-sealed")
	require.NoError(t, err)
	expectSealedPayloadScan(mock, col, 10, sealedPayloadRows(col,
		"c1", "legacy-one",
		"c2", "legacy-two",
		"c3", v1))

	var code int
	out := captureStdout(t, func() {
		code = encryptSealedPayloadsWithDB(context.Background(), sqlDB, encryptor, []crypto.SealedPayloadColumn{col},
			crypto.SealedPayloadBackfillOptions{BatchSize: 10, DryRun: true})
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "would_encrypt=2")
	assert.Contains(t, out, "WOULD-SEAL guliangiam.pay_channel_configs.config_sealed pk=c1 class=unrecognized stage=encrypt reason=plaintext_candidate")
	assert.Contains(t, out, "WOULD-SEAL guliangiam.pay_channel_configs.config_sealed pk=c2 class=unrecognized stage=encrypt reason=plaintext_candidate")
	assert.Contains(t, out, "(dry-run, no rows written)")
	assert.NotContains(t, out, "legacy-one")
	assert.NotContains(t, out, "legacy-two")
	t.Log(out)
	require.NoError(t, mock.ExpectationsWereMet(), "dry-run must issue no UPDATE")
}

func TestEncryptSealedPayloads_FailureCountAndMaxFailures(t *testing.T) {
	col := crypto.SealedPayloadColumns[1]

	run := func(t *testing.T, maxFailures int) (int, string) {
		t.Helper()
		sqlDB, mock, encryptor := newBackfillDB(t)
		expectSealedPayloadScan(mock, col, 10, sealedPayloadRows(col,
			"c1", "legacy-one",
			"c2", "legacy-two"))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"UPDATE %s SET %s = $1 WHERE %s = $2 AND %s = $3", col.Table, col.Column, col.PKColumn, col.Column))).
			WithArgs(sqlmock.AnyArg(), "c1", "legacy-one").
			WillReturnError(sqlmock.ErrCancelled)
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"UPDATE %s SET %s = $1 WHERE %s = $2 AND %s = $3", col.Table, col.Column, col.PKColumn, col.Column))).
			WithArgs(sqlmock.AnyArg(), "c2", "legacy-two").
			WillReturnError(sqlmock.ErrCancelled)

		var code int
		out := captureStdout(t, func() {
			code = encryptSealedPayloadsWithDB(context.Background(), sqlDB, encryptor, []crypto.SealedPayloadColumn{col},
				crypto.SealedPayloadBackfillOptions{BatchSize: 10, MaxFailures: maxFailures})
		})
		assert.Contains(t, out, "selected=2 encrypted=0")
		assert.Contains(t, out, "failed=2")
		assert.NotContains(t, out, "legacy-one")
		assert.NotContains(t, out, "legacy-two")
		require.NoError(t, mock.ExpectationsWereMet())
		return code, out
	}

	t.Run("below threshold exits zero", func(t *testing.T) {
		code, _ := run(t, 5)
		assert.Equal(t, 0, code)
	})
	t.Run("above threshold exits non-zero", func(t *testing.T) {
		code, _ := run(t, 1)
		assert.Equal(t, 1, code)
	})
}

func TestEncryptSealedPayloads_RejectsInvalidFlags(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	assert.Equal(t, 2, runEncryptSealedPayloads([]string{"-dry-run", "-verify"}))
	assert.Equal(t, 2, runEncryptSealedPayloads([]string{"-force", "-verify"}))
	assert.Equal(t, 2, runEncryptSealedPayloads([]string{"-batch-size", "0"}))
	assert.Equal(t, 2, runEncryptSealedPayloads([]string{"-batch-size", fmt.Sprintf("%d", pagination.MaxPageSize+1)}))
	assert.Equal(t, 2, runEncryptSealedPayloads([]string{"-max-failures", "-1"}))
	assert.Equal(t, 2, runEncryptSealedPayloads([]string{"-table", "unknown_table"}))
}

func TestEncryptSealedPayloads_FailsWithoutKeyBeforeConnecting(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 1, runEncryptSealedPayloads([]string{"-dsn", "postgres://example.invalid/db"}))
}

func TestEncryptSealedPayloads_CommandIsRecognized(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("POSTGRES_DSN", "")
	assert.Equal(t, 2, runWithArgs([]string{"encrypt-sealed-payloads"}))
}

// TestEncryptSealedPayloadsPostgres is real-PostgreSQL evidence for GM-R5a.
// It is skipped unless GIAM_TEST_PG_DSN points at an isolated, already-migrated
// disposable database whose role bypasses FORCE RLS (migration role/owner/
// superuser). Never point this at the shared development database.
func TestEncryptSealedPayloadsPostgres(t *testing.T) {
	dsn := os.Getenv("GIAM_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("GIAM_TEST_PG_DSN is not set; skipping real PostgreSQL sealed payload backfill test (use an isolated, already-migrated disposable database)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	t.Cleanup(pool.Close)

	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	encryptor := crypto.NewFieldEncryptor()
	col := crypto.SealedPayloadColumns[0]

	orgID := uuid.NewString()
	slug := "gm-r5a-" + strings.ReplaceAll(orgID, "-", "")
	_, err = pool.Exec(ctx,
		`INSERT INTO guliangiam.organizations (id, slug, display_name) VALUES ($1::uuid, $2, $3)`,
		orgID, slug, "GM-R5a sealed backfill")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx,
			`DELETE FROM guliangiam.llm_providers WHERE organization_id = $1::uuid`, orgID)
		_, _ = pool.Exec(cleanupCtx,
			`DELETE FROM guliangiam.organizations WHERE id = $1::uuid`, orgID)
	})

	plaintexts := []string{"legacy-api-key-a", "legacy-api-key-b"}
	ids := []string{uuid.NewString(), uuid.NewString()}
	for i, id := range ids {
		_, err = pool.Exec(ctx,
			`INSERT INTO guliangiam.llm_providers
			 (id, organization_id, name, provider_type, base_url, api_key_sealed, api_key_fingerprint)
			 VALUES ($1::uuid, $2::uuid, $3, 'openai', 'https://example.invalid', $4, '')`,
			id, orgID, fmt.Sprintf("gm-r5a-provider-%d", i), plaintexts[i])
		require.NoError(t, err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	var code int
	out := captureStdout(t, func() {
		code = encryptSealedPayloadsWithDB(ctx, sqlDB, encryptor, []crypto.SealedPayloadColumn{col},
			crypto.SealedPayloadBackfillOptions{BatchSize: 10})
	})
	require.Equal(t, 0, code)
	require.Contains(t, out, "selected=2 encrypted=2")

	for i, id := range ids {
		var stored string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT api_key_sealed FROM guliangiam.llm_providers WHERE id = $1::uuid`, id).Scan(&stored))
		require.True(t, crypto.IsEnvelopeToken(stored), "the backfill must store a v1 envelope")
		header, _, _ := strings.Cut(stored, ":")
		require.False(t, strings.HasSuffix(header, "-aad"), "api_key_sealed must never be upgraded to v2")
		opened, decryptErr := encryptor.DecryptField(stored)
		require.NoError(t, decryptErr)
		require.Equal(t, plaintexts[i], opened)
	}

	captureStdout(t, func() {
		code = encryptSealedPayloadsWithDB(ctx, sqlDB, encryptor, []crypto.SealedPayloadColumn{col},
			crypto.SealedPayloadBackfillOptions{BatchSize: 10})
	})
	require.Equal(t, 0, code, "a re-run must select no plaintext")

	out = captureStdout(t, func() {
		code = encryptSealedPayloadsWithDB(ctx, sqlDB, encryptor, []crypto.SealedPayloadColumn{col},
			crypto.SealedPayloadBackfillOptions{BatchSize: 10, Verify: true})
	})
	require.Equal(t, 0, code)
	require.Contains(t, out, "plaintext_remaining=0")
	require.Contains(t, out, "aad_mismatch=0")
	require.Contains(t, out, "verify OK")
}
