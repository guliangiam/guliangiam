package crypto

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sealedV1ValueArg matches a stored sealed payload that is a v1 envelope
// (never v2/AAD) and decrypts back to the expected plaintext with DecryptField.
type sealedV1ValueArg struct {
	plaintext string
	encryptor *FieldEncryptor
}

func (a sealedV1ValueArg) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok || !IsEnvelopeToken(stored) || isV2EnvelopeToken(stored) || strings.Contains(stored, "-aad:") {
		return false
	}
	opened, err := a.encryptor.DecryptField(stored)
	return err == nil && opened == a.plaintext
}

func TestSealedPayloadColumns_RegistryIsSeparateFromV2Columns(t *testing.T) {
	require.Len(t, SealedPayloadColumns, 2)

	keys := map[string]bool{}
	for _, col := range SealedPayloadColumns {
		require.NoError(t, col.Validate())
		keys[col.Table+"."+col.Column] = true
	}
	assert.True(t, keys["guliangiam.llm_providers.api_key_sealed"])
	assert.True(t, keys["guliangiam.pay_channel_configs.config_sealed"])

	for _, sealed := range SealedPayloadColumns {
		for _, col := range DefaultEncryptedColumns {
			assert.NotEqual(t, col.Table+"."+col.Column, sealed.Table+"."+sealed.Column,
				"a v1 sealed payload must never enter the AAD-bound v2 registry")
		}
	}
}

func TestSealedPayloadColumn_ValidateRejectsUnsafeIdentifier(t *testing.T) {
	col := SealedPayloadColumn{Table: "guliangiam.llm_providers; DROP TABLE", Column: "api_key_sealed", PKColumn: "id"}
	require.Error(t, col.Validate())
}

func TestClassifySealedPayload_Generations(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	v1, err := encryptor.EncryptField("sk-sealed")
	require.NoError(t, err)
	v0 := directKEKToken(t, encryptor, "sk-sealed")
	v2, err := encryptor.EncryptColumn(sealedAAD(SealedPayloadColumns[0].Table, SealedPayloadColumns[0].Column, "p1"), "sk-sealed")
	require.NoError(t, err)

	otherKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("\x11", 32)))
	t.Setenv("ENCRYPTION_KEY_CURRENT", otherKey)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	other := NewFieldEncryptor()
	foreignV1, err := other.EncryptField("sk-sealed")
	require.NoError(t, err)

	cases := []struct {
		name    string
		value   string
		want    EncryptedPayloadClass
		wantErr error
	}{
		{name: "empty is skipped", value: "", want: PayloadClassSkipped},
		{name: "v1 is legacy but readable", value: v1, want: PayloadClassV1Legacy},
		{name: "v0 is legacy but readable", value: v0, want: PayloadClassV0Legacy},
		{name: "plaintext is unrecognized", value: "sk-plaintext", want: PayloadClassUnrecognized, wantErr: ErrNotEnvelope},
		{name: "v2 is unrecognized", value: v2, want: PayloadClassUnrecognized, wantErr: ErrSealedPayloadV2},
		{name: "foreign v1 is unreadable", value: foreignV1, want: PayloadClassUnreadable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ClassifySealedPayload(encryptor, tc.value)
			assert.Equal(t, tc.want, got)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else if tc.want != PayloadClassUnreadable {
				assert.NoError(t, err)
			}
		})
	}
}

// A sealed payload must be re-sealed as v1 even when the source token is a
// stale v1/v0; upgrading it to v2 would break the application's DecryptField.
func TestRotateSealedPayload_KeepsV1(t *testing.T) {
	legacy, rotating := newRotationEncryptor(t)
	stale, err := legacy.EncryptField("sk-sealed")
	require.NoError(t, err)

	rotated, changed, err := rotating.RotateSealedPayload(stale)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, IsEnvelopeToken(rotated))
	assert.False(t, isV2EnvelopeToken(rotated))
	assert.NotContains(t, rotated, "-aad:")
	opened, err := rotating.DecryptField(rotated)
	require.NoError(t, err)
	assert.Equal(t, "sk-sealed", opened)

	current, err := rotating.EncryptField("sk-current")
	require.NoError(t, err)
	same, changed, err := rotating.RotateSealedPayload(current)
	require.NoError(t, err)
	assert.False(t, changed, "a v1 token at the current KEK is already as current as v1 can be")
	assert.Equal(t, current, same)
}

func TestRotateSealedPayload_RefusesV2(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	v2, err := encryptor.EncryptColumn(sealedAAD(SealedPayloadColumns[0].Table, SealedPayloadColumns[0].Column, "p1"), "sk-sealed")
	require.NoError(t, err)

	rotated, changed, err := encryptor.RotateSealedPayload(v2)
	require.ErrorIs(t, err, ErrSealedPayloadV2)
	assert.False(t, changed)
	assert.Empty(t, rotated)
}

// A current-header v1 token must be authenticated before it is skipped; a
// tampered payload is an error, not "already current".
func TestRotateSealedPayload_AuthenticatesBeforeSkipping(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	token, err := encryptor.EncryptField("sk-sealed")
	require.NoError(t, err)

	last := token[len(token)-1]
	replacement := byte('A')
	if last == 'A' {
		replacement = 'B'
	}
	tampered := token[:len(token)-1] + string(replacement)

	_, changed, err := encryptor.RotateSealedPayload(tampered)
	require.Error(t, err)
	assert.False(t, changed)
}

func TestRotateSealedPayload_EmptyStaysEmpty(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	rotated, changed, err := encryptor.RotateSealedPayload("")
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Empty(t, rotated)
}

func TestScanSealedPayloads_ClassifiesAndCapsQueue(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := SealedPayloadColumns[0]
	v1, err := encryptor.EncryptField("sk-sealed")
	require.NoError(t, err)
	v2, err := encryptor.EncryptColumn(sealedAAD(col.Table, col.Column, "p5"), "sk-sealed")
	require.NoError(t, err)

	source := &fakePayloadSource{pages: map[string][]PayloadRow{
		"": {
			{PK: "p1", Value: payloadValue(v1)},
			{PK: "p2", Value: payloadValue("sk-plaintext")},
			{PK: "p3", Value: sql.NullString{}},
			{PK: "p4", Value: payloadValue("")},
			{PK: "p5", Value: payloadValue(v2)},
		},
	}}

	report, err := ScanSealedPayloads(context.Background(), source, encryptor, []SealedPayloadColumn{col},
		EncryptedPayloadScanOptions{BatchSize: 10, MaxIssues: 1, QueueOnlyFailures: true})
	require.NoError(t, err)
	require.Len(t, report.Columns, 1)
	stat := report.Columns[0]
	assert.Equal(t, int64(5), stat.Scanned)
	assert.Equal(t, int64(2), stat.Skipped)
	assert.Equal(t, int64(1), stat.V1Legacy)
	assert.Equal(t, int64(2), stat.Unrecognized, "plaintext and v2 are both unrecognized")
	assert.Zero(t, stat.V2Current, "a sealed payload scan never reports v2 as current")
	assert.Equal(t, int64(2), report.IssueCount)
	require.Len(t, report.Issues, 1, "the queue is capped but IssueCount is not")
	assert.True(t, report.IssuesTruncated)
	assert.Equal(t, "p2", report.Issues[0].PK)
	assert.Equal(t, PayloadClassUnrecognized, report.Issues[0].Class)
	require.Len(t, source.requests, 1)
	assert.Equal(t, PayloadBatch{
		Table: col.Table, Column: col.Column, PKColumn: col.PKColumn, AfterPK: "", Limit: 10,
	}, source.requests[0])
}

// The checkpoint must stay before the first failed row even when later rows
// were rewritten, so the next run retries that row instead of skipping it.
func TestReencryptJob_RunForSealedPayloads_CheckpointStopsBeforeFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	legacy, rotating := newRotationEncryptor(t)
	first, err := legacy.EncryptField("sk-first")
	require.NoError(t, err)
	third, err := legacy.EncryptField("sk-third")
	require.NoError(t, err)

	col := SealedPayloadColumns[0]
	path := filepath.Join(t.TempDir(), "reencrypt.checkpoint")
	job := NewReencryptJob(db, rotating, path)

	mock.ExpectQuery("SELECT id, api_key_sealed FROM guliangiam.llm_providers ORDER BY id ASC LIMIT 500").
		WillReturnRows(sqlmock.NewRows([]string{"id", "api_key_sealed"}).
			AddRow("p1", first).
			AddRow("p2", "sk-plaintext").
			AddRow("p3", third))
	mock.ExpectExec("UPDATE guliangiam.llm_providers SET api_key_sealed").
		WithArgs(sealedV1ValueArg{plaintext: "sk-first", encryptor: rotating}, "p1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE guliangiam.llm_providers SET api_key_sealed").
		WithArgs(sealedV1ValueArg{plaintext: "sk-third", encryptor: rotating}, "p3").
		WillReturnResult(sqlmock.NewResult(0, 1))

	stats, err := job.RunForSealedPayloads(context.Background(), []SealedPayloadColumn{col})
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.Scanned)
	assert.Equal(t, int64(2), stats.Updated)
	assert.Equal(t, int64(1), stats.Errors)
	require.Len(t, stats.Failures, 1)
	assert.Equal(t, "p2", stats.Failures[0].PK)
	assert.Equal(t, PayloadClassUnrecognized, stats.Failures[0].Class)
	assert.Equal(t, "reseal", stats.Failures[0].Stage)
	assert.True(t, stats.Completed)
	assert.FileExists(t, path, "a failed run must keep the checkpoint")
	assert.Contains(t, readCheckpoint(t, path), "guliangiam.llm_providers.api_key_sealed=p1")
	require.NoError(t, mock.ExpectationsWereMet())
}

// -max-failures caps the printed queue, never the error count.
func TestReencryptJob_RunForSealedPayloads_MaxFailuresQueue(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	_, rotating := newRotationEncryptor(t)
	col := SealedPayloadColumns[1]

	mock.ExpectQuery("SELECT id, config_sealed FROM guliangiam.pay_channel_configs ORDER BY id ASC LIMIT 500").
		WillReturnRows(sqlmock.NewRows([]string{"id", "config_sealed"}).
			AddRow("p1", "plain-1").
			AddRow("p2", "plain-2").
			AddRow("p3", "plain-3"))

	job := NewReencryptJob(db, rotating, filepath.Join(t.TempDir(), "reencrypt.checkpoint"))
	job.SetMaxFailures(1)
	stats, err := job.RunForSealedPayloads(context.Background(), []SealedPayloadColumn{col})
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.Errors)
	require.Len(t, stats.Failures, 1, "the queue is capped")
	assert.True(t, stats.FailuresTruncated)
	assert.Equal(t, "p1", stats.Failures[0].PK)
	assert.Zero(t, stats.Updated)
	require.NoError(t, mock.ExpectationsWereMet(), "plaintext never produces an UPDATE")
}

func TestSealedPayloadAction_NeverSuggestsV2(t *testing.T) {
	for _, class := range []EncryptedPayloadClass{
		PayloadClassV1Legacy, PayloadClassV0Legacy, PayloadClassUnrecognized, PayloadClassUnreadable,
	} {
		action := SealedPayloadAction(class)
		assert.NotContains(t, action, "AAD-bound v2", "sealed payload repair must never suggest v2: %s", class)
	}
}

// sealedAAD builds the v2 row binding a column would use, so fixtures can
// prove a v2 token in a sealed payload column is reported, never rewritten.
func sealedAAD(table, column, pk string) AADContext {
	return AADContext{Table: table, Column: column, PKValue: pk}
}
