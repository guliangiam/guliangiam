package idv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func sealedSessionToken(t *testing.T, verificationID, plaintext string) string {
	t.Helper()
	sealed, err := crypto.GlobalFieldEncryptor().EncryptColumn(identityVerificationAAD(verificationID), plaintext)
	require.NoError(t, err)
	require.True(t, crypto.IsEnvelopeToken(sealed))
	return sealed
}

func strPtr(value string) *string { return &value }

func TestSealSessionToken(t *testing.T) {
	t.Run("nil_stays_nil", func(t *testing.T) {
		sealed, err := sealSessionToken("ver-1", nil)
		require.NoError(t, err)
		assert.Nil(t, sealed)
	})

	t.Run("empty_stays_empty", func(t *testing.T) {
		sealed, err := sealSessionToken("ver-1", strPtr(""))
		require.NoError(t, err)
		require.NotNil(t, sealed)
		assert.Empty(t, *sealed)
	})

	t.Run("seals_with_row_aad", func(t *testing.T) {
		sealed, err := sealSessionToken("ver-1", strPtr("sess-token-abc"))
		require.NoError(t, err)
		require.NotNil(t, sealed)
		require.True(t, crypto.IsEnvelopeToken(*sealed))
		assert.NotContains(t, *sealed, "sess-token-abc")
		header, _, _ := strings.Cut(*sealed, ":")
		assert.True(t, strings.HasSuffix(header, "-aad"), "v2 AAD envelope expected")
	})

	t.Run("oversized_envelope_is_refused", func(t *testing.T) {
		token := strings.Repeat("s", 120)
		_, err := sealSessionToken("ver-1", &token)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), token)
	})
}

func TestOpenSessionToken(t *testing.T) {
	t.Run("sealed_roundtrip", func(t *testing.T) {
		sealed := sealedSessionToken(t, "ver-1", "sess-token-abc")
		opened, err := openSessionToken("ver-1", "org-1", &sealed)
		require.NoError(t, err)
		require.NotNil(t, opened)
		assert.Equal(t, "sess-token-abc", *opened)
	})

	t.Run("legacy_plaintext_falls_back_with_minimal_warn", func(t *testing.T) {
		logs := captureWarnings(t)
		stored := "sess-legacy-plaintext"
		opened, err := openSessionToken("ver-2", "org-2", &stored)
		require.NoError(t, err)
		require.NotNil(t, opened)
		assert.Equal(t, "sess-legacy-plaintext", *opened)
		assert.Contains(t, logs.String(), "ver-2")
		assert.Contains(t, logs.String(), "org-2")
		assert.NotContains(t, logs.String(), "sess-legacy-plaintext", "the WARN must never include the token")
	})

	t.Run("aad_binding_rejects_another_row", func(t *testing.T) {
		sealed := sealedSessionToken(t, "other-verification", "sess-token-abc")
		_, err := openSessionToken("ver-3", "org-3", &sealed)
		require.Error(t, err)
		assert.ErrorIs(t, err, crypto.ErrAADMismatch)
	})

	t.Run("nil_and_empty_stay_unset", func(t *testing.T) {
		opened, err := openSessionToken("ver-4", "org-4", nil)
		require.NoError(t, err)
		assert.Nil(t, opened)

		empty := ""
		opened, err = openSessionToken("ver-4", "org-4", &empty)
		require.NoError(t, err)
		require.NotNil(t, opened)
		assert.Empty(t, *opened)
	})
}

func TestRepository_Create_SealsSessionToken(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	var captured []any
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
		captured = args
		return pgconn.NewCommandTag("INSERT 1"), nil
	}}
	repo := NewRepository(db)
	verification := &domain.Verification{
		ID:             "ver-1",
		OrganizationID: "org-1",
		Provider:       "jumio",
		Status:         domain.StatusPending,
		SessionToken:   strPtr("sess-token-abc"),
		RawResult:      json.RawMessage(`{}`),
		InitiatedAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	require.NoError(t, repo.Create(context.Background(), verification))

	require.Len(t, captured, 27)
	stored, ok := captured[6].(*string)
	require.True(t, ok, "session_token is a nullable column; the sealed value must stay a *string")
	require.NotNil(t, stored)
	require.True(t, crypto.IsEnvelopeToken(*stored))
	require.NotContains(t, *stored, "sess-token-abc")
	opened, err := crypto.GlobalFieldEncryptor().DecryptColumn(identityVerificationAAD("ver-1"), *stored)
	require.NoError(t, err)
	require.Equal(t, "sess-token-abc", opened)
}

func TestRepository_Create_NilSessionTokenStaysNil(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
		captured = args
		return pgconn.NewCommandTag("INSERT 1"), nil
	}}
	repo := NewRepository(db)
	verification := &domain.Verification{ID: "ver-2", OrganizationID: "org-2", Provider: "jumio"}
	require.NoError(t, repo.Create(context.Background(), verification))

	require.Len(t, captured, 27)
	assert.Nil(t, captured[6], "an absent session_token must stay NULL, not become an encryption of empty")
	assert.Nil(t, captured[14], "an absent id_number must stay NULL in the sealed target")
	require.NotNil(t, captured[18], "the implicit {} raw_result must be sealed, not left legacy")
}

// ─── A-6.3C-1: PII dual write / sealed-read fallback ─────────────────────────

func TestRepository_Create_SealsPIIColumns(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
		captured = args
		return pgconn.NewCommandTag("INSERT 1"), nil
	}}
	repo := NewRepository(db)
	idNumber := "AB1234567"
	raw := json.RawMessage(`{"document":{"number":"AB1234567"}}`)
	verification := &domain.Verification{
		ID:             "ver-pii",
		OrganizationID: "org-pii",
		Provider:       "jumio",
		Status:         domain.StatusPending,
		IDNumber:       &idNumber,
		RawResult:      raw,
		InitiatedAt:    time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	require.NoError(t, repo.Create(context.Background(), verification))
	require.Len(t, captured, 27)

	assert.Equal(t, &idNumber, captured[13], "the legacy plaintext column stays authoritative for the upgrade window")
	require.Equal(t, raw, captured[17], "raw_result plaintext is written unchanged")

	idNumberSealed := requireSealedString(t, captured[14])
	idNumberCol, err := identityVerificationSealedColumn(identityVerificationColumnIDNumber)
	require.NoError(t, err)
	opened, err := crypto.GlobalFieldEncryptor().DecryptColumn(idNumberCol.AAD(verification.ID), *idNumberSealed)
	require.NoError(t, err)
	assert.Equal(t, idNumber, opened)
	assert.NotContains(t, *idNumberSealed, idNumber)

	rawResultSealed := requireSealedString(t, captured[18])
	rawCol, err := identityVerificationSealedColumn(identityVerificationColumnRawResult)
	require.NoError(t, err)
	openedRaw, err := crypto.GlobalFieldEncryptor().DecryptColumn(rawCol.AAD(verification.ID), *rawResultSealed)
	require.NoError(t, err)
	assert.Equal(t, string(raw), openedRaw)
}

func TestRepository_Update_SealsPIIColumns(t *testing.T) {
	var captured []any
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
		captured = args
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}}
	repo := NewRepository(db)
	idNumber := "AB1234567"
	raw := json.RawMessage(`{"document":"AB1234567"}`)
	verification := &domain.Verification{
		ID:             "ver-pii",
		OrganizationID: "org-pii",
		Status:         domain.StatusApproved,
		IDNumber:       &idNumber,
		RawResult:      raw,
		UpdatedAt:      time.Now(),
	}
	require.NoError(t, repo.Update(context.Background(), verification))
	require.Len(t, captured, 18)

	assert.Equal(t, &idNumber, captured[6])
	require.Equal(t, raw, captured[10])

	idNumberSealed := requireSealedString(t, captured[7])
	idNumberCol, err := identityVerificationSealedColumn(identityVerificationColumnIDNumber)
	require.NoError(t, err)
	opened, err := crypto.GlobalFieldEncryptor().DecryptColumn(idNumberCol.AAD(verification.ID), *idNumberSealed)
	require.NoError(t, err)
	assert.Equal(t, idNumber, opened)

	rawResultSealed := requireSealedString(t, captured[11])
	rawCol, err := identityVerificationSealedColumn(identityVerificationColumnRawResult)
	require.NoError(t, err)
	openedRaw, err := crypto.GlobalFieldEncryptor().DecryptColumn(rawCol.AAD(verification.ID), *rawResultSealed)
	require.NoError(t, err)
	assert.Equal(t, string(raw), openedRaw)
}

func TestScanOne_PrefersSealedOverPlaintext(t *testing.T) {
	idNumberPlain := "LEGACY-ID"
	rawPlain := json.RawMessage(`{"legacy":true}`)
	idNumberSealed := sealedLegacyValue(t, identityVerificationColumnIDNumber, "ver-scan", "SEALED-ID")
	rawSealed := sealedLegacyValue(t, identityVerificationColumnRawResult, "ver-scan", `{"sealed":true}`)

	values := baseVerificationRowValues()
	values[17] = &idNumberPlain
	values[18] = &idNumberSealed
	values[21] = rawPlain
	values[22] = &rawSealed

	verification, err := scanOne(pgxRowValues(values))
	require.NoError(t, err)
	require.NotNil(t, verification.IDNumber)
	require.NotNil(t, verification.RawResult)
	assert.Equal(t, "SEALED-ID", *verification.IDNumber)
	assert.JSONEq(t, `{"sealed":true}`, string(verification.RawResult))
}

func TestScanOne_FallsBackWhenSealedIsNull(t *testing.T) {
	logs := captureWarnings(t)
	idNumberPlain := "LEGACY-ID"
	rawPlain := json.RawMessage(`{"legacy":true}`)

	values := baseVerificationRowValues()
	values[17] = &idNumberPlain
	values[18] = nil
	values[21] = rawPlain
	values[22] = nil

	verification, err := scanOne(pgxRowValues(values))
	require.NoError(t, err)
	require.NotNil(t, verification.IDNumber)
	require.NotNil(t, verification.RawResult)
	assert.Equal(t, "LEGACY-ID", *verification.IDNumber)
	assert.JSONEq(t, `{"legacy":true}`, string(verification.RawResult))

	assert.Contains(t, logs.String(), "backfill-pii-sealed-columns")
	assert.NotContains(t, logs.String(), "LEGACY-ID", "the fallback WARN must never include the plaintext")
	assert.NotContains(t, logs.String(), `"legacy":true`, "the fallback WARN must never include the raw result")
}

func TestScanOne_RejectsAADMismatch(t *testing.T) {
	t.Run("id_number", func(t *testing.T) {
		idNumberPlain := "LEGACY-ID"
		foreign := sealedLegacyValue(t, identityVerificationColumnIDNumber, "other-row", "SEALED-ID")
		values := baseVerificationRowValues()
		values[17] = &idNumberPlain
		values[18] = &foreign

		_, err := scanOne(pgxRowValues(values))
		require.Error(t, err)
		assert.ErrorIs(t, err, crypto.ErrAADMismatch)
	})

	t.Run("raw_result", func(t *testing.T) {
		rawPlain := json.RawMessage(`{"legacy":true}`)
		foreign := sealedLegacyValue(t, identityVerificationColumnRawResult, "other-row", `{"sealed":true}`)
		values := baseVerificationRowValues()
		values[21] = rawPlain
		values[22] = &foreign

		_, err := scanOne(pgxRowValues(values))
		require.Error(t, err)
		assert.ErrorIs(t, err, crypto.ErrAADMismatch)
	})

	t.Run("wrong_column", func(t *testing.T) {
		idNumberPlain := "LEGACY-ID"
		foreign := sealedLegacyValue(t, identityVerificationColumnRawResult, "ver-scan", `{"sealed":true}`)
		values := baseVerificationRowValues()
		values[17] = &idNumberPlain
		values[18] = &foreign

		_, err := scanOne(pgxRowValues(values))
		require.Error(t, err)
		assert.ErrorIs(t, err, crypto.ErrAADMismatch)
	})
}

func TestIdentityVerificationSealedColumnsMatchRegistry(t *testing.T) {
	for _, source := range []string{identityVerificationColumnIDNumber, identityVerificationColumnRawResult} {
		col, err := identityVerificationSealedColumn(source)
		require.NoError(t, err)
		assert.Equal(t, identityVerificationTable, col.Table)
		assert.Equal(t, source, col.SourceColumn)
		assert.Equal(t, "id", col.PKColumn)
	}
}

func requireSealedString(t *testing.T, value any) *string {
	t.Helper()
	sealed, ok := value.(*string)
	require.True(t, ok, "sealed target must be written as a nullable *string")
	require.NotNil(t, sealed)
	require.True(t, crypto.IsEnvelopeToken(*sealed), "sealed target must be an envelope token")
	header, _, _ := strings.Cut(*sealed, ":")
	require.True(t, strings.HasSuffix(header, "-aad"), "sealed target must be v2 (AAD-bound)")
	return sealed
}

func sealedLegacyValue(t *testing.T, sourceColumn, verificationID, plaintext string) string {
	t.Helper()
	col, err := identityVerificationSealedColumn(sourceColumn)
	require.NoError(t, err)
	sealed, err := crypto.GlobalFieldEncryptor().EncryptColumn(col.AAD(verificationID), plaintext)
	require.NoError(t, err)
	return sealed
}

// baseVerificationRowValues returns a 27-column row in query order, with every
// optional column unset except the implicit {} raw_result.
func baseVerificationRowValues() []any {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	userID := "user-scan"
	return []any{
		"ver-scan", "org-scan", &userID, "jumio", domain.StatusApproved,
		nil, nil, nil,
		nil, nil, nil, nil,
		nil, nil, nil, nil,
		nil, nil, nil, nil, nil,
		json.RawMessage(`{}`), nil,
		now, nil, now, now,
	}
}

type pgxRowValues []any

func (r pgxRowValues) Scan(dest ...any) error {
	if len(dest) != len(r) {
		return fmt.Errorf("scan destination count %d does not match value count %d", len(dest), len(r))
	}
	for i, value := range r {
		if value == nil {
			continue
		}
		if err := assignScanValue(dest[i], value); err != nil {
			return fmt.Errorf("scan column %d: %w", i, err)
		}
	}
	return nil
}

func assignScanValue(destination, source any) error {
	target := reflect.ValueOf(destination)
	if target.Kind() != reflect.Pointer || target.IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer")
	}
	return setScanValue(target.Elem(), reflect.ValueOf(source))
}

func setScanValue(target, source reflect.Value) error {
	if target.Kind() == reflect.Pointer {
		if source.Type().AssignableTo(target.Type()) {
			target.Set(source)
			return nil
		}
		value := reflect.New(target.Type().Elem())
		if err := setScanValue(value.Elem(), source); err != nil {
			return err
		}
		target.Set(value)
		return nil
	}
	if source.Type().AssignableTo(target.Type()) {
		target.Set(source)
		return nil
	}
	if source.Type().ConvertibleTo(target.Type()) {
		target.Set(source.Convert(target.Type()))
		return nil
	}
	return fmt.Errorf("cannot assign %s to %s", source.Type(), target.Type())
}

var _ pgx.Row = pgxRowValues(nil)
