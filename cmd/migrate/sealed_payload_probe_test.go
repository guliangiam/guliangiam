package main

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// The probe must report the v1/v2 distribution with the row AAD and must not
// issue any write. A v2 token stored under a different primary key is counted
// as unreadable, proving the probe authenticates row identity instead of just
// reading the token header.
func TestScanSealedPayloads_ReportsAADDistributionWithoutWriting(t *testing.T) {
	sqlDB, mock, encryptor := newBackfillDB(t)

	llm := crypto.SealedPayloadColumns[0]
	llmTarget := crypto.EncryptedColumn{Table: llm.Table, Column: llm.Column, PKColumn: llm.PKColumn}
	v1, err := encryptor.EncryptField("sk-v1")
	require.NoError(t, err)
	v2, err := encryptor.EncryptColumn(crypto.LLMProviderAPIKeySealedColumn.AAD("p2"), "sk-v2")
	require.NoError(t, err)
	moved, err := encryptor.EncryptColumn(crypto.LLMProviderAPIKeySealedColumn.AAD("p9"), "sk-moved")
	require.NoError(t, err)

	expectPayloadScan(mock, llmTarget, 10, sqlmock.NewRows([]string{"id", "api_key_sealed"}).
		AddRow("p1", v1).
		AddRow("p2", v2).
		AddRow("p3", moved))

	pay := crypto.SealedPayloadColumns[1]
	payTarget := crypto.EncryptedColumn{Table: pay.Table, Column: pay.Column, PKColumn: pay.PKColumn}
	expectPayloadScan(mock, payTarget, 10, sqlmock.NewRows([]string{"id", "config_sealed"}).
		AddRow("c1", "plaintext-config"))

	var code int
	out := captureStdout(t, func() {
		code = scanSealedPayloadsWithDB(context.Background(), sqlDB, encryptor, 10)
	})

	assert.Equal(t, 0, code)
	assert.Contains(t, out, "guliangiam.llm_providers.api_key_sealed: scanned=3 skipped=0 current-v2=1 stale-v2=0 legacy-v1=1 legacy-v0=0 unrecognized=0 unreadable=1")
	assert.Contains(t, out, "guliangiam.pay_channel_configs.config_sealed: scanned=1 skipped=0 current-v2=0 stale-v2=0 legacy-v1=0 legacy-v0=0 unrecognized=1 unreadable=0")
	assert.Contains(t, out, "sealed payload scan summary: scanned=4 skipped=0 current-v2=1 stale-v2=0 legacy-v1=1 legacy-v0=0 unrecognized=1 unreadable=1 (read-only, no rows written)")
	require.NoError(t, mock.ExpectationsWereMet(), "the probe must never issue a write")
}
