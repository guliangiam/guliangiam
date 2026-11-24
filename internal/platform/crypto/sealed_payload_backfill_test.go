package crypto

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
	"github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// fakeSealedPayloadBackfillSource serves rows keyed by table.column and records
// every update, so the backfill contract is tested without SQL mocks. The SQL
// templates themselves are covered by internal/infra/postgres/cryptopayload.
type fakeSealedPayloadBackfillSource struct {
	rows      map[string][]PayloadRow
	requests  []PayloadBatch
	updates   []SealedPayloadWrite
	affected  int64
	updateErr error
}

func newFakeSealedPayloadBackfillSource(rows map[string][]PayloadRow) *fakeSealedPayloadBackfillSource {
	return &fakeSealedPayloadBackfillSource{rows: rows, affected: 1}
}

func (f *fakeSealedPayloadBackfillSource) FetchPayloadBatch(_ context.Context, batch PayloadBatch) ([]PayloadRow, error) {
	f.requests = append(f.requests, batch)
	key := batch.Table + "." + batch.Column
	var out []PayloadRow
	for _, row := range f.rows[key] {
		if batch.AfterPK != "" && row.PK <= batch.AfterPK {
			continue
		}
		out = append(out, row)
		if len(out) >= batch.Limit {
			break
		}
	}
	return out, nil
}

func (f *fakeSealedPayloadBackfillSource) UpdateSealedPayload(_ context.Context, write SealedPayloadWrite) (int64, error) {
	f.updates = append(f.updates, write)
	return f.affected, f.updateErr
}

func sealedRowsKey(col SealedPayloadColumn) string {
	return col.Table + "." + col.Column
}

func nullableValue(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}

func nullValue() sql.NullString {
	return sql.NullString{}
}

func registeredSealedPayload(t *testing.T, table string) SealedPayloadColumn {
	t.Helper()
	for _, registered := range SealedPayloadColumns {
		if registered.Table == table {
			return registered
		}
	}
	t.Fatalf("sealed payload column in table %s is not registered", table)
	return SealedPayloadColumn{}
}

// A new registration must be picked up without editing the command: the
// registry itself is the domain selection.
func TestSealedPayloadBackfill_TableDrivenCoversEveryRegisteredColumn(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	llm := registeredSealedPayload(t, LLMProviderAPIKeySealedColumn.Table)
	pay := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)

	source := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(llm): {{PK: "p1", Value: nullableValue("sk-plain")}},
		sealedRowsKey(pay): {{PK: "c1", Value: nullableValue(`{"apiKey":"x"}`)}},
	})

	report, err := BackfillSealedPayloads(context.Background(), source, encryptor, SealedPayloadColumns,
		SealedPayloadBackfillOptions{BatchSize: 10})
	require.NoError(t, err)
	require.Len(t, report.Columns, len(SealedPayloadColumns))

	seen := map[string]bool{}
	for _, stat := range report.Columns {
		seen[stat.Table+"."+stat.Column] = true
		assert.Equal(t, int64(1), stat.Selected, "%s must be scanned from the registry", stat.Column)
		assert.Equal(t, int64(1), stat.Encrypted)
	}
	assert.True(t, seen[llm.Table+"."+llm.Column])
	assert.True(t, seen[pay.Table+"."+pay.Column], "pay_channel_configs.config_sealed must be covered")
	require.Len(t, source.updates, 2)
}

// The written pass must seal only non-envelope values and must not turn a
// readable v1/v0/v2 value into a second ciphertext.
func TestSealedPayloadBackfill_SealsPlaintextAndSkipsReadableValues(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)

	v1, err := encryptor.EncryptField("v1-secret")
	require.NoError(t, err)
	v2, err := encryptor.EncryptColumn(col.AAD("c4"), "v2-secret")
	require.NoError(t, err)
	v0 := directKEKToken(t, encryptor, "v0-secret")

	source := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(col): {
			{PK: "c1", Value: nullableValue(`{"apiKey":"plain"}`)},
			{PK: "c2", Value: nullableValue(v1)},
			{PK: "c3", Value: nullableValue(v0)},
			{PK: "c4", Value: nullableValue(v2)},
			{PK: "c5", Value: nullableValue("")},
			{PK: "c6", Value: nullValue()},
		},
	})

	report, err := BackfillSealedPayloads(context.Background(), source, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: 10})
	require.NoError(t, err)
	stat := report.Columns[0]
	assert.Equal(t, int64(6), stat.Scanned)
	assert.Equal(t, int64(1), stat.Selected)
	assert.Equal(t, int64(1), stat.Encrypted)
	assert.Equal(t, int64(5), stat.Skipped)
	assert.Equal(t, int64(1), stat.LegacyV1)
	assert.Equal(t, int64(1), stat.LegacyV0)
	assert.Equal(t, int64(1), stat.V2Current)
	assert.Equal(t, int64(1), stat.Plaintext)
	assert.Equal(t, int64(0), stat.Failed)
	require.Len(t, source.updates, 1, "only the plaintext row may be written")
	assert.Equal(t, "c1", source.updates[0].PK)
	assert.Equal(t, `{"apiKey":"plain"}`, source.updates[0].Expected)

	opened, err := encryptor.DecryptColumn(col.AAD("c1"), source.updates[0].Sealed)
	require.NoError(t, err)
	assert.Equal(t, `{"apiKey":"plain"}`, opened)
	header, _, _ := cutSealedHeader(source.updates[0].Sealed)
	assert.False(t, header, "the backfill must keep the v1 generation (no AAD)")
}

// Re-running the command after a successful pass must not produce a second
// ciphertext: the sealed value is readable and therefore skipped.
func TestSealedPayloadBackfill_SecondRunSkipsSealedRowsWithoutUpdate(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)

	plaintext := `{"apiKey":"plain"}`
	first := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(col): {{PK: "c1", Value: nullableValue(plaintext)}},
	})
	firstReport, err := BackfillSealedPayloads(context.Background(), first, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), firstReport.Columns[0].Encrypted)
	require.Len(t, first.updates, 1)

	sealed := first.updates[0].Sealed
	second := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(col): {{PK: "c1", Value: nullableValue(sealed)}},
	})
	secondReport, err := BackfillSealedPayloads(context.Background(), second, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), secondReport.Columns[0].Selected, "a re-run must select no plaintext")
	assert.Equal(t, int64(0), secondReport.Columns[0].Encrypted)
	assert.Equal(t, int64(1), secondReport.Columns[0].Skipped)
	assert.Empty(t, second.updates, "a re-run over sealed rows must issue no UPDATE")
}

// -force re-seals readable legacy v1/v0 values at the current KEK but never
// downgrades an AAD-authenticated v2 token.
func TestSealedPayloadBackfill_ForceResealsLegacyButNeverDowngradesV2(t *testing.T) {
	previous := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x5a}, 32))
	t.Setenv("ENCRYPTION_KEY", previous)
	t.Setenv("ENCRYPTION_KEY_CURRENT", "")
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", "")
	t.Setenv("IS_PRODUCTION", "false")
	legacy := NewFieldEncryptor()
	staleV1, err := legacy.EncryptField("stale-v1")
	require.NoError(t, err)
	staleV0 := directKEKToken(t, legacy, "stale-v0")

	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_CURRENT", cryptotest.Key)
	t.Setenv("ENCRYPTION_KEY_PREVIOUS", previous)
	encryptor := NewFieldEncryptor()

	col := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)
	v2, err := encryptor.EncryptColumn(col.AAD("c3"), "v2-secret")
	require.NoError(t, err)

	source := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(col): {
			{PK: "c1", Value: nullableValue(staleV1)},
			{PK: "c2", Value: nullableValue(staleV0)},
			{PK: "c3", Value: nullableValue(v2)},
		},
	})

	report, err := BackfillSealedPayloads(context.Background(), source, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: 10, Force: true})
	require.NoError(t, err)
	stat := report.Columns[0]
	assert.Equal(t, int64(2), stat.Resealed)
	assert.Equal(t, int64(0), stat.Encrypted)
	assert.Equal(t, int64(2), stat.LegacyV1+stat.LegacyV0)
	require.Len(t, source.updates, 2, "v2 must not be rewritten")
	for _, update := range source.updates {
		assert.NotEqual(t, "c3", update.PK)
		header, _, _ := cutSealedHeader(update.Sealed)
		assert.False(t, header, "-force must keep the v1 generation")
	}
	assert.Equal(t, "c1", source.updates[0].PK)
	assert.Equal(t, "c2", source.updates[1].PK)
}

// -dry-run reports the would-seal queue and never calls the write port.
func TestSealedPayloadBackfill_DryRunListsWouldSealWithoutWriting(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)
	v1, err := encryptor.EncryptField("already-sealed")
	require.NoError(t, err)

	source := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(col): {
			{PK: "c1", Value: nullableValue("plain-1")},
			{PK: "c2", Value: nullableValue("plain-2")},
			{PK: "c3", Value: nullableValue(v1)},
		},
	})

	report, err := BackfillSealedPayloads(context.Background(), source, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: 10, DryRun: true})
	require.NoError(t, err)
	stat := report.Columns[0]
	assert.Equal(t, int64(2), stat.Selected)
	assert.Equal(t, int64(0), stat.Encrypted)
	assert.Equal(t, int64(1), stat.Skipped)
	require.Len(t, stat.Failures, 2)
	assert.Equal(t, "plaintext_candidate", stat.Failures[0].Reason)
	assert.Equal(t, "c1", stat.Failures[0].PK)
	assert.Empty(t, source.updates, "dry-run must never call the write port")
}

// -verify accepts readable v1 and AAD-authenticated v2 values, and reports a
// moved v2 token (AAD mismatch) plus plaintext/unreadable rows.
func TestSealedPayloadBackfill_VerifyReportsAADMismatchAndPlaintext(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)

	v1, err := encryptor.EncryptField("v1-secret")
	require.NoError(t, err)
	v2, err := encryptor.EncryptColumn(col.AAD("c2"), "v2-secret")
	require.NoError(t, err)
	moved, err := encryptor.EncryptColumn(col.AAD("c9"), "v2-moved")
	require.NoError(t, err)
	corruptEnvelope := "v1-aes:aaaa:bbbb:cccc"

	source := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(col): {
			{PK: "c1", Value: nullableValue(v1)},
			{PK: "c2", Value: nullableValue(v2)},
			{PK: "c3", Value: nullableValue(moved)},
			{PK: "c4", Value: nullableValue("plaintext")},
			{PK: "c5", Value: nullableValue(corruptEnvelope)},
		},
	})

	report, err := BackfillSealedPayloads(context.Background(), source, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: 10, Verify: true})
	require.NoError(t, err)
	stat := report.Columns[0]
	assert.Equal(t, int64(1), stat.LegacyV1)
	assert.Equal(t, int64(1), stat.V2Current)
	assert.Equal(t, int64(1), stat.Plaintext)
	assert.Equal(t, int64(2), stat.Unreadable)
	assert.Equal(t, int64(1), stat.AADMismatch)
	assert.Equal(t, int64(0), stat.Encrypted)
	assert.Empty(t, source.updates, "verify must never call the write port")

	reasons := map[string]int{}
	for _, failure := range stat.Failures {
		reasons[failure.Reason]++
	}
	assert.Equal(t, 1, reasons["aad_mismatch"])
	assert.Equal(t, 1, reasons["plaintext_remaining"])
	assert.Equal(t, 1, reasons["decrypt_failed"])
}

func TestSealedPayloadBackfill_VerifyAcceptsRegisteredAADAndSkipsEmpty(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)
	v2, err := encryptor.EncryptColumn(col.AAD("c1"), "v2-secret")
	require.NoError(t, err)
	v1, err := encryptor.EncryptField("v1-secret")
	require.NoError(t, err)

	source := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(col): {
			{PK: "c1", Value: nullableValue(v2)},
			{PK: "c2", Value: nullableValue(v1)},
			{PK: "c3", Value: nullableValue("")},
			{PK: "c4", Value: nullValue()},
		},
	})

	report, err := BackfillSealedPayloads(context.Background(), source, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: 10, Verify: true})
	require.NoError(t, err)
	stat := report.Columns[0]
	assert.Equal(t, int64(1), stat.V2Current)
	assert.Equal(t, int64(1), stat.LegacyV1)
	assert.Equal(t, int64(2), stat.Skipped)
	assert.Equal(t, int64(0), stat.Plaintext)
	assert.Equal(t, int64(0), stat.AADMismatch)
	assert.Equal(t, int64(0), stat.Unreadable)
}

// A concurrent writer that replaced the value between the scan and the CAS
// must win; the backfill counts the row as skipped and writes nothing.
func TestSealedPayloadBackfill_CASLossIsSkippedNotOverwritten(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)
	source := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(col): {{PK: "c1", Value: nullableValue("plain")}},
	})
	source.affected = 0

	report, err := BackfillSealedPayloads(context.Background(), source, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: 10})
	require.NoError(t, err)
	stat := report.Columns[0]
	assert.Equal(t, int64(1), stat.Selected)
	assert.Equal(t, int64(0), stat.Encrypted)
	assert.Equal(t, int64(1), stat.Skipped)
	require.Len(t, source.updates, 1, "the CAS is attempted once but affects no row")
}

func TestSealedPayloadBackfill_UpdateFailuresAreCountedAndQueueIsCapped(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)
	source := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(col): {
			{PK: "c1", Value: nullableValue("plain-1")},
			{PK: "c2", Value: nullableValue("plain-2")},
		},
	})
	source.updateErr = assert.AnError

	report, err := BackfillSealedPayloads(context.Background(), source, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: 10, MaxFailures: 1})
	require.NoError(t, err)
	stat := report.Columns[0]
	assert.Equal(t, int64(2), stat.Selected)
	assert.Equal(t, int64(2), stat.Failed)
	assert.Len(t, stat.Failures, 1)
	assert.True(t, stat.FailuresTruncated)
	assert.Equal(t, "update_failed", stat.Failures[0].Reason)
}

func TestSealedPayloadBackfill_RejectsInvalidOptions(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	source := newFakeSealedPayloadBackfillSource(nil)
	col := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)

	_, err := BackfillSealedPayloads(context.Background(), source, encryptor, []SealedPayloadColumn{col},
		SealedPayloadBackfillOptions{DryRun: true, Verify: true})
	require.Error(t, err)

	_, err = BackfillSealedPayloads(context.Background(), source, encryptor, []SealedPayloadColumn{col},
		SealedPayloadBackfillOptions{Verify: true, Force: true})
	require.Error(t, err)

	unsafe := col
	unsafe.Table = "guliangiam.pay_channel_configs; DROP TABLE"
	_, err = BackfillSealedPayloads(context.Background(), source, encryptor, []SealedPayloadColumn{unsafe},
		SealedPayloadBackfillOptions{})
	require.Error(t, err)
}

// The command must reuse the single pagination boundary instead of a new
// scan-specific limit.
func TestSealedPayloadBackfill_BatchSizeUsesPaginationBound(t *testing.T) {
	encryptor := newPayloadEncryptor(t)
	col := registeredSealedPayload(t, PayChannelConfigSealedColumn.Table)
	source := newFakeSealedPayloadBackfillSource(map[string][]PayloadRow{
		sealedRowsKey(col): {{PK: "c1", Value: nullableValue("plain")}},
	})

	_, err := BackfillSealedPayloads(context.Background(), source, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: 0, DryRun: true})
	require.NoError(t, err)
	require.Len(t, source.requests, 1)
	assert.Equal(t, pagination.DefaultPageSize, source.requests[0].Limit)

	_, err = BackfillSealedPayloads(context.Background(), source, encryptor,
		[]SealedPayloadColumn{col}, SealedPayloadBackfillOptions{BatchSize: pagination.MaxPageSize + 1, DryRun: true})
	require.NoError(t, err)
	require.Len(t, source.requests, 2)
	assert.Equal(t, pagination.MaxPageSize, source.requests[1].Limit)
}

// The backfill must never hand-build an AADContext: every binding comes from
// LegacySealedColumn.AAD, the same table|column|pk constructor the PII backfill
// uses.
func TestSealedPayloadBackfill_AADDerivesFromLegacySealedColumnRule(t *testing.T) {
	for _, col := range SealedPayloadColumns {
		legacy := LegacySealedColumn{Table: col.Table, TargetColumn: col.Column, PKColumn: col.PKColumn}
		aad := col.AAD("row-1")
		assert.Equal(t, legacy.AAD("row-1"), aad)
		assert.Equal(t, col.Table, aad.Table)
		assert.Equal(t, col.Column, aad.Column)
		assert.Equal(t, "row-1", aad.PKValue)
	}
}

// cutSealedHeader reports whether the token is the AAD-bound (v2) generation.
func cutSealedHeader(token string) (hasAAD bool, header string, ok bool) {
	header, _, ok = strings.Cut(token, ":")
	if !ok {
		return false, "", false
	}
	return strings.HasSuffix(header, "-aad"), header, true
}
