package crypto

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// fakePayloadSource serves preloaded keyset pages and records every request so
// the scan tests pin cursor semantics without SQL mocks; the SQL templates
// themselves are covered by internal/infra/postgres/cryptopayload.
type fakePayloadSource struct {
	pages    map[string][]PayloadRow
	err      error
	errAfter string
	requests []PayloadBatch
}

func (f *fakePayloadSource) FetchPayloadBatch(_ context.Context, batch PayloadBatch) ([]PayloadRow, error) {
	f.requests = append(f.requests, batch)
	if f.err != nil && batch.AfterPK == f.errAfter {
		return nil, f.err
	}
	return f.pages[batch.AfterPK], nil
}

func payloadValue(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}

func newPayloadEncryptor(t *testing.T) *FieldEncryptor {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	return NewFieldEncryptor()
}

// directKEKToken reproduces the pre-envelope v0 format: base64(nonce+ciphertext)
// encrypted directly with the current KEK.
func directKEKToken(t *testing.T, encryptor *FieldEncryptor, plaintext string) string {
	t.Helper()
	gcm := encryptor.envelope.keksByAlg[encryptor.envelope.algorithm][encryptor.envelope.current]
	nonce := make([]byte, gcm.NonceSize())
	// #nosec G407 -- fixed nonce reproduces the legacy on-disk format for a test
	// fixture only; it is never used by production encryption.
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed)
}

func TestClassifyEncryptedPayload(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := EncryptedColumn{Table: tableProviders, Column: "client_secret", PKColumn: "id"}

	current, err := encryptor.EncryptColumn(col.AAD("p1"), "current-secret")
	require.NoError(t, err)
	v1, err := encryptor.EncryptField("legacy-secret")
	require.NoError(t, err)
	v0 := directKEKToken(t, encryptor, "legacy-v0-secret")
	otherRow, err := encryptor.EncryptColumn(col.AAD("p2"), "other-row-secret")
	require.NoError(t, err)

	v0Only, rotating := newRotationEncryptor(t)
	stale, err := v0Only.EncryptColumn(col.AAD("p1"), "stale-secret")
	require.NoError(t, err)

	cases := []struct {
		name      string
		value     string
		encryptor *FieldEncryptor
		want      EncryptedPayloadClass
	}{
		{name: "empty skips", value: "", encryptor: encryptor, want: PayloadClassSkipped},
		{name: "current v2", value: current, encryptor: encryptor, want: PayloadClassV2Current},
		{name: "stale v2", value: stale, encryptor: rotating, want: PayloadClassV2Stale},
		{name: "v1 legacy", value: v1, encryptor: encryptor, want: PayloadClassV1Legacy},
		{name: "v0 legacy", value: v0, encryptor: encryptor, want: PayloadClassV0Legacy},
		{name: "plaintext is unrecognized", value: "plaintext-secret", encryptor: encryptor, want: PayloadClassUnrecognized},
		{name: "cross-row v2 is unreadable", value: otherRow, encryptor: encryptor, want: PayloadClassUnreadable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := ClassifyEncryptedPayload(tc.encryptor, col, "p1", tc.value)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEncryptedPayloadClassAdvice(t *testing.T) {
	assert.True(t, PayloadClassV1Legacy.NeedsRotation())
	assert.True(t, PayloadClassV0Legacy.NeedsRotation())
	assert.True(t, PayloadClassV2Stale.NeedsRotation())
	assert.True(t, PayloadClassUnrecognized.NeedsBackfill())
	assert.True(t, PayloadClassUnreadable.IsBroken())
	assert.Contains(t, PayloadClassV1Legacy.Action(), "migrate reencrypt")
	assert.Contains(t, PayloadClassUnrecognized.Action(), "backfill-encrypted-columns")
	assert.Contains(t, PayloadClassUnreadable.Action(), "restore")
}

func TestScanEncryptedPayloads_ClassifiesAndCapsQueue(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := EncryptedColumn{Table: tableProviders, Column: "client_secret", PKColumn: "id"}
	current, err := encryptor.EncryptColumn(col.AAD("p4"), "current-secret")
	require.NoError(t, err)
	v1, err := encryptor.EncryptField("legacy-secret")
	require.NoError(t, err)

	source := &fakePayloadSource{pages: map[string][]PayloadRow{
		"": {
			{PK: "p1", Value: payloadValue("plaintext-one")},
			{PK: "p2", Value: payloadValue(v1)},
			{PK: "p3", Value: sql.NullString{}},
			{PK: "p4", Value: payloadValue(current)},
			{PK: "p5", Value: payloadValue("")},
			{PK: "p6", Value: payloadValue("plaintext-two")},
		},
	}}

	report, err := ScanEncryptedPayloads(context.Background(), source, encryptor, []EncryptedColumn{col},
		EncryptedPayloadScanOptions{BatchSize: 10, MaxIssues: 2})
	require.NoError(t, err)
	require.Len(t, report.Columns, 1)
	stat := report.Columns[0]
	assert.Equal(t, int64(6), stat.Scanned)
	assert.Equal(t, int64(2), stat.Skipped)
	assert.Equal(t, int64(1), stat.V2Current)
	assert.Equal(t, int64(1), stat.V1Legacy)
	assert.Equal(t, int64(2), stat.Unrecognized)

	assert.Equal(t, int64(3), report.IssueCount)
	require.Len(t, report.Issues, 2, "the queue is capped but IssueCount is not")
	assert.True(t, report.IssuesTruncated)
	assert.Equal(t, "p1", report.Issues[0].PK)
	assert.Equal(t, PayloadClassUnrecognized, report.Issues[0].Class)
	assert.Equal(t, "p2", report.Issues[1].PK)
	assert.Equal(t, PayloadClassV1Legacy, report.Issues[1].Class)
	require.Len(t, source.requests, 1)
	assert.Equal(t, PayloadBatch{
		Table: col.Table, Column: col.Column, PKColumn: col.PKColumn, AfterPK: "", Limit: 10,
	}, source.requests[0])
}

// Backfill verify only queues plaintext/unreadable rows; legacy envelopes are
// counted but must not consume the bounded queue.
func TestScanEncryptedPayloads_QueueOnlyFailuresSkipsLegacy(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := EncryptedColumn{Table: tableProviders, Column: "client_secret", PKColumn: "id"}
	v1, err := encryptor.EncryptField("legacy-secret")
	require.NoError(t, err)
	current, err := encryptor.EncryptColumn(col.AAD("p3"), "current-secret")
	require.NoError(t, err)

	source := &fakePayloadSource{pages: map[string][]PayloadRow{
		"": {
			{PK: "p1", Value: payloadValue(v1)},
			{PK: "p2", Value: payloadValue("plaintext-secret")},
			{PK: "p3", Value: payloadValue(current)},
		},
	}}

	report, err := ScanEncryptedPayloads(context.Background(), source, encryptor, []EncryptedColumn{col},
		EncryptedPayloadScanOptions{BatchSize: 10, MaxIssues: 1, QueueOnlyFailures: true})
	require.NoError(t, err)
	require.Len(t, report.Columns, 1)
	assert.Equal(t, int64(1), report.Columns[0].V1Legacy)
	assert.Equal(t, int64(1), report.Columns[0].Unrecognized)
	assert.Equal(t, int64(1), report.IssueCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, "p2", report.Issues[0].PK)
	assert.False(t, report.IssuesTruncated)
}

// An all-NULL/empty first batch must advance the keyset cursor instead of
// ending the scan before later rows are classified.
func TestScanEncryptedPayloads_SkipsNullEmptyAndPaginates(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := EncryptedColumn{Table: "guliangiam.webhooks", Column: "secret", PKColumn: "id"}

	source := &fakePayloadSource{pages: map[string][]PayloadRow{
		"": {
			{PK: "a", Value: sql.NullString{}},
			{PK: "b", Value: payloadValue("")},
		},
		"b": {{PK: "c", Value: payloadValue("plaintext-secret")}},
	}}

	report, err := ScanEncryptedPayloads(context.Background(), source, encryptor, []EncryptedColumn{col},
		EncryptedPayloadScanOptions{BatchSize: 2})
	require.NoError(t, err)
	require.Len(t, report.Columns, 1)
	assert.Equal(t, int64(3), report.Columns[0].Scanned)
	assert.Equal(t, int64(2), report.Columns[0].Skipped)
	assert.Equal(t, int64(1), report.Columns[0].Unrecognized)
	assert.Equal(t, int64(1), report.IssueCount)
	require.Len(t, report.Issues, 1)
	assert.Equal(t, "c", report.Issues[0].PK)
	require.Len(t, source.requests, 2, "the all-skipped first page must not end the scan")
	assert.Equal(t, "", source.requests[0].AfterPK)
	assert.Equal(t, "b", source.requests[1].AfterPK, "the cursor advances to the last PK of the full page")
	assert.Equal(t, 2, source.requests[1].Limit)
}

// A source failure must abort the column and propagate unchanged, leaving the
// report without the partial column stat (same contract as the SQL-backed
// implementation).
func TestScanEncryptedPayloads_PropagatesSourceError(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := EncryptedColumn{Table: "guliangiam.webhooks", Column: "secret", PKColumn: "id"}
	sentinel := errors.New("scan failed")
	source := &fakePayloadSource{
		pages: map[string][]PayloadRow{
			"": {{PK: "a", Value: payloadValue("plaintext-secret")}},
		},
		err:      sentinel,
		errAfter: "a",
	}

	report, err := ScanEncryptedPayloads(context.Background(), source, encryptor, []EncryptedColumn{col},
		EncryptedPayloadScanOptions{BatchSize: 1})
	require.ErrorIs(t, err, sentinel)
	require.Len(t, source.requests, 2)
	assert.Equal(t, "a", source.requests[1].AfterPK)
	assert.Empty(t, report.Columns, "a failed column is not reported as complete")
}

func TestScanEncryptedPayloads_RejectsNilSource(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := EncryptedColumn{Table: tableProviders, Column: "client_secret", PKColumn: "id"}

	_, err := ScanEncryptedPayloads(context.Background(), nil, encryptor, []EncryptedColumn{col},
		EncryptedPayloadScanOptions{})
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInternal, appErr.Code)
}

// A failed row must be queued with its PK, and the checkpoint must stay before
// that PK so the next run retries instead of skipping it.
func TestReencryptJob_FailureQueueAndCheckpointRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	_, rotating := newRotationEncryptor(t)
	col := EncryptedColumn{Table: tableUserMfaConfig, Column: "totp_secret", PKColumn: pkUserID}
	v1a, err := rotating.EncryptField("a-secret")
	require.NoError(t, err)
	v1b, err := rotating.EncryptField("b-secret")
	require.NoError(t, err)
	v1c, err := rotating.EncryptField("c-secret")
	require.NoError(t, err)
	currentC, err := rotating.EncryptColumn(col.AAD("c"), "c-secret")
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "reencrypt.checkpoint")
	job := NewReencryptJob(db, rotating, path)
	job.batchSize = 2
	job.SetMaxFailures(1)

	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config ORDER BY user_id ASC LIMIT 2").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}).
			AddRow("a", v1a).
			AddRow("b", "plaintext-b"))
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs(encryptedValueArg{plaintext: "a-secret", encryptor: rotating, table: tableUserMfaConfig, column: "totp_secret", pk: "a"}, "a").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config WHERE user_id > \\$1").
		WithArgs("b").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}).AddRow("c", v1c))
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs(encryptedValueArg{plaintext: "c-secret", encryptor: rotating, table: tableUserMfaConfig, column: "totp_secret", pk: "c"}, "c").
		WillReturnResult(sqlmock.NewResult(0, 1))

	stats, err := job.RunFor(context.Background(), []EncryptedColumn{col})
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.Scanned)
	assert.Equal(t, int64(2), stats.Updated)
	assert.Equal(t, int64(1), stats.Errors)
	require.Len(t, stats.Failures, 1)
	assert.Equal(t, "b", stats.Failures[0].PK)
	assert.Equal(t, PayloadClassUnrecognized, stats.Failures[0].Class)
	assert.Equal(t, "rotate", stats.Failures[0].Stage)
	assert.True(t, stats.Completed)
	assert.FileExists(t, path, "a failed run must keep the checkpoint")
	assert.Contains(t, readCheckpoint(t, path), "guliangiam.user_mfa_config.totp_secret=a")

	// The next run resumes after a, retries b (now fixed to a v1 token) and
	// authenticates c as already-current v2.
	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config WHERE user_id > \\$1").
		WithArgs("a").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}).
			AddRow("b", v1b).
			AddRow("c", currentC))
	mock.ExpectExec("UPDATE guliangiam.user_mfa_config SET totp_secret").
		WithArgs(encryptedValueArg{plaintext: "b-secret", encryptor: rotating, table: tableUserMfaConfig, column: "totp_secret", pk: "b"}, "b").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT user_id, totp_secret FROM guliangiam.user_mfa_config WHERE user_id > \\$1").
		WithArgs("c").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "totp_secret"}))

	second, err := job.RunFor(context.Background(), []EncryptedColumn{col})
	require.NoError(t, err)
	assert.Equal(t, int64(2), second.Scanned)
	assert.Equal(t, int64(1), second.Updated)
	assert.Equal(t, int64(1), second.Skipped)
	assert.Zero(t, second.Errors)
	require.NoError(t, job.ClearCheckpoint())
	assert.NoFileExists(t, path)
	require.NoError(t, mock.ExpectationsWereMet())
}

func readCheckpoint(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
