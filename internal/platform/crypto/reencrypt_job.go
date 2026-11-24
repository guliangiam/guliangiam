package crypto

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// safeIdentifier matches SQL identifiers used for schema-qualified table names
// and unqualified column names. Rejects quotes, comments, operators, etc.
var safeIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

// EncryptedColumn describes a table column whose values are envelope-encrypted
// and may need re-encryption during key rotation.
type EncryptedColumn struct {
	Table    string // fully-qualified table, e.g. "guliangiam.webhooks"
	Column   string // encrypted column name, e.g. "secret"
	PKColumn string // primary key column name, e.g. "id"
}

// Validate returns nil iff all three identifiers are safe for SQL construction.
// Call this before using any EncryptedColumn in a fmt.Sprintf SQL fragment.
func (c EncryptedColumn) Validate() error {
	return validateIdentifiers(c.Table, c.Column, c.PKColumn)
}

// validateIdentifiers is the single identifier rule shared by the v2 column and
// v1 sealed-payload registries.
func validateIdentifiers(table, column, pkColumn string) error {
	for _, name := range []struct{ n, k string }{{table, "table"}, {column, "column"}, {pkColumn, "pk-column"}} {
		if !safeIdentifier.MatchString(name.n) {
			return apperrors.Newf(apperrors.CodeUnprocessableEntity, "unsafe %s identifier: %q", name.k, name.n)
		}
	}
	return nil
}

// AAD builds the v2 row binding for one value of this column. The primary key
// text comes from the column's own PKColumn, which is never assumed to be "id".
func (c EncryptedColumn) AAD(pk string) AADContext {
	return AADContext{Table: c.Table, Column: c.Column, PKValue: pk}
}

// tableOrganizationAuthConfig 是加密列注册表中组织敏感字段的限定名。
// master_password / master_verification_code / default_password 位于
// organization_auth_config（PK organization_id），不是 organizations 表。
const tableOrganizationAuthConfig = "guliangiam.organization_auth_config"

// pkOrganizationID 是 organization_auth_config 的主键列。
const pkOrganizationID = "organization_id"

// tableProviders 是 provider 凭据列的限定表名。
const tableProviders = "guliangiam.providers"

// tableUserMfaConfig 是用户 MFA 配置子表（PK user_id），承载 TOTP 种子。
const tableUserMfaConfig = "guliangiam.user_mfa_config"

// pkUserID 是 user_mfa_config 的主键列。
const pkUserID = "user_id"

// DefaultEncryptedColumns lists every envelope-encrypted column in the current
// schema. Adding encryption to a repository column requires a matching entry
// here; scripts/check-encrypted-columns.py verifies that each entry resolves
// against contracts/schema, that ProviderCredentialColumns is a subset, and
// that every non-shared EncryptedFieldMapper package owns a registered column.
// The script cannot infer a newly encrypted column from code.
//
// The provider credential columns were plaintext before B1b and joined this
// registry only after the backfill shipped. Rotation re-encrypts an existing
// token; a plaintext row is reported as an error and left untouched, which is
// why `migrate encrypt-provider-secrets` must run before a key rotation.
//
// radius_configs.shared_secret, captcha_providers.secret_key and
// certs.private_key/access_secret joined in V8 (GM-G5): the repositories seal
// new writes, but rows written before V8 are still plaintext. They stay out of
// ProviderCredentialColumns because that list is the B1b provider backfill
// input, not a generic plaintext sweep.
//
// user_mfa_config.totp_secret joined in GM-G2 (already TEXT, no schema change).
// Pre-GM-G2 rows are plaintext and reads fail closed until BackfillMFASecrets
// runs; it stays out of ProviderCredentialColumns, which is provider-only.
//
// guliangiam.keys.private_key joined in GM R6 (already TEXT NULL, no schema
// change). It holds tenant signing material: the keys repositories seal new
// writes into a v2 envelope bound to (table, column, id) and reject a pre-R6
// plaintext row until `migrate backfill-encrypted-columns` seals it. NULL and
// empty values mean "no private material" and must stay untouched.
var DefaultEncryptedColumns = []EncryptedColumn{
	{Table: tableOrganizationAuthConfig, Column: "master_password", PKColumn: pkOrganizationID},
	{Table: tableOrganizationAuthConfig, Column: "master_verification_code", PKColumn: pkOrganizationID},
	{Table: tableOrganizationAuthConfig, Column: "default_password", PKColumn: pkOrganizationID},
	{Table: "guliangiam.webhooks", Column: "secret", PKColumn: "id"},
	{Table: "guliangiam.syncers", Column: "password", PKColumn: "id"},
	{Table: "guliangiam.syncers", Column: "ssh_password", PKColumn: "id"},
	{Table: "guliangiam.ldaps", Column: "bind_password", PKColumn: "id"},
	{Table: "guliangiam.agents", Column: "token", PKColumn: "id"},
	{Table: "guliangiam.adapters", Column: "password", PKColumn: "id"},
	{Table: "guliangiam.adapters", Column: "db_password", PKColumn: "id"},
	{Table: "guliangiam.radius_configs", Column: "shared_secret", PKColumn: "id"},
	{Table: "guliangiam.captcha_providers", Column: "secret_key", PKColumn: "id"},
	{Table: "guliangiam.certs", Column: "private_key", PKColumn: "id"},
	{Table: "guliangiam.certs", Column: "access_secret", PKColumn: "id"},
	{Table: tableProviders, Column: "client_secret", PKColumn: "id"},
	{Table: tableProviders, Column: "client_secret2", PKColumn: "id"},
	{Table: tableProviders, Column: "password", PKColumn: "id"},
	{Table: tableProviders, Column: "app_secret", PKColumn: "id"},
	{Table: "guliangiam.notification_providers", Column: "access_token", PKColumn: "id"},
	{Table: tableUserMfaConfig, Column: "totp_secret", PKColumn: pkUserID},
	{Table: "guliangiam.keys", Column: "private_key", PKColumn: "id"},
	// connectors.bearer_token_sealed 保留历史 `_sealed` 后缀，但仓储用
	// EncryptColumn 写入 v2（AAD 绑定 table/column/connector id）。因此登记在
	// 本表而不是 SealedPayloadColumns，轮换时走 AAD 绑定路径。
	{Table: "guliangiam.connectors", Column: "bearer_token_sealed", PKColumn: "id"},
	// A-6.1: face_id_providers.access_key_secret 复用原 VARCHAR(255) 列存 v2
	// 信封，AAD 绑定 (table, column, id)。旧明文行由
	// `migrate encrypt-faceid-secrets` 回填；回填前读路径允许回退并标记轮换。
	{Table: "guliangiam.face_id_providers", Column: "access_key_secret", PKColumn: "id"},
	// A-6.2: 两列独立 bearer/session 令牌同样复用原 VARCHAR(255) 列存 v2 信封
	// （AAD 绑定 table/column/id），没有新增 `_sealed` 列。存量明文由
	// `migrate encrypt-credential-secrets` 回填；回填前读路径回退并写 WARN。
	// 列宽上限（AES-256 约 92 字符明文 / SM4 约 107 字符）登记在 A-6.2 回执。
	{Table: "guliangiam.mcp_servers", Column: "token", PKColumn: "id"},
	{Table: "guliangiam.identity_verifications", Column: "session_token", PKColumn: "id"},
	// A-6.3A/A-6.3B: 两表的 `_sealed` 列承载 v2（AAD 绑定 table/column/PK）
	// 信封；读写走 EncryptColumn/DecryptColumn，所以登记在本表而不是
	// SealedPayloadColumns（后者是 v1、无 AAD）。存量明文由
	// `migrate backfill-pii-sealed-columns` 回填，NULL 表示尚未封存。
	{Table: "guliangiam.user_collections", Column: "managed_accounts_sealed", PKColumn: pkUserID},
	{Table: "guliangiam.user_collections", Column: "mfa_items_sealed", PKColumn: pkUserID},
	{Table: "guliangiam.user_collections", Column: "mfa_accounts_sealed", PKColumn: pkUserID},
	{Table: "guliangiam.identity_verifications", Column: "id_number_sealed", PKColumn: "id"},
	{Table: "guliangiam.identity_verifications", Column: "raw_result_sealed", PKColumn: "id"},
	{Table: "guliangiam.user_profiles", Column: "id_card_sealed", PKColumn: pkUserID},
	{Table: "guliangiam.user_profiles", Column: "passport_no_sealed", PKColumn: pkUserID},
	{Table: "guliangiam.user_profiles", Column: "driver_license_no_sealed", PKColumn: pkUserID},
}

// ProviderCredentialColumns are the provider credential columns that stored
// plaintext before B1b. They are the only input of the
// `migrate encrypt-provider-secrets` backfill; columns that were encrypted
// earlier (organizations, webhooks, ...) must not be touched because they may
// legitimately contain legacy v0 tokens.
var ProviderCredentialColumns = []EncryptedColumn{
	{Table: tableProviders, Column: "client_secret", PKColumn: "id"},
	{Table: tableProviders, Column: "client_secret2", PKColumn: "id"},
	{Table: tableProviders, Column: "password", PKColumn: "id"},
	{Table: tableProviders, Column: "app_secret", PKColumn: "id"},
	{Table: "guliangiam.notification_providers", Column: "access_token", PKColumn: "id"},
}

// CredentialSecretColumns are the standalone bearer/session tokens sealed in
// A-6.2. They are the input of `migrate encrypt-credential-secrets` and are
// deliberately not part of ProviderCredentialColumns: that list is the B1b
// provider backfill contract and must not change shape behind operators.
//
// The columns keep their historical VARCHAR(255) definitions, so an envelope
// longer than 255 characters cannot be stored. Writes refuse an oversized
// envelope (the repositories return an internal error naming only the length);
// widening the columns to TEXT is a follow-up schema change.
var CredentialSecretColumns = []EncryptedColumn{
	{Table: "guliangiam.mcp_servers", Column: "token", PKColumn: "id"},
	{Table: "guliangiam.identity_verifications", Column: "session_token", PKColumn: "id"},
}

// ReencryptJob re-encrypts all envelope-encrypted columns with the current
// KEK version. It supports resumption via a checkpoint file so a long-running
// rotation can be interrupted and continued.
//
// The job is idempotent: every row is rewritten into the AAD-bound v2
// generation, and a v2 row already at the current KEK/algorithm is authenticated
// against its table/column/PK and skipped (no UPDATE). v1 is never treated as
// current, so a first pass upgrades legacy ciphertext.
type ReencryptJob struct {
	db            *sql.DB
	encryptor     *FieldEncryptor
	batchSize     int
	maxFailures   int
	maxRowsPerRun int
	checkpoint    *CheckpointStore
	mu            sync.Mutex
	stats         ReencryptStats
}

// ReencryptStats holds the result of a re-encryption run. Errors counts every
// failed row (plus per-column scan failures); Failures is the bounded queue an
// operator can replay. FailuresTruncated reports that the queue hit its cap.
type ReencryptStats struct {
	Scanned           int64
	Updated           int64
	Skipped           int64
	Errors            int64
	Failures          []EncryptedPayloadFailure
	FailuresTruncated bool
	Duration          time.Duration
	Completed         bool
}

// SetMaxRowsPerRun bounds how many rows one RunFor call may scan. It is used by
// the online rotation worker so a tick stays short; cursors are still persisted,
// so the next tick resumes where this one stopped. 0 = unlimited.
func (j *ReencryptJob) SetMaxRowsPerRun(n int) {
	if n >= 0 {
		j.maxRowsPerRun = n
	}
}

// SetBatchSize overrides the per-SQL-batch row count (default 500).
func (j *ReencryptJob) SetBatchSize(n int) {
	if n > 0 {
		j.batchSize = n
	}
}

// SetMaxFailures caps the per-column failure queue (0 = unlimited).
func (j *ReencryptJob) SetMaxFailures(n int) {
	if n < 0 {
		return
	}
	j.maxFailures = n
}

// NewReencryptJob creates a re-encryption job. checkpointPath is the file used
// to persist progress; pass "" to disable checkpointing.
func NewReencryptJob(db *sql.DB, encryptor *FieldEncryptor, checkpointPath string) *ReencryptJob {
	j := &ReencryptJob{
		db:        db,
		encryptor: encryptor,
		batchSize: 500,
	}
	if checkpointPath != "" {
		j.checkpoint = NewCheckpointStore(checkpointPath)
	}
	return j
}

// Run executes the re-encryption job across all default encrypted columns and
// clears the checkpoint only when every column finished without errors.
func (j *ReencryptJob) Run(ctx context.Context) (ReencryptStats, error) {
	stats, err := j.RunFor(ctx, DefaultEncryptedColumns)
	if err == nil && stats.Completed && stats.Errors == 0 {
		_ = j.ClearCheckpoint()
	}
	return stats, err
}

// RunFor executes the re-encryption job for the given columns.
func (j *ReencryptJob) RunFor(ctx context.Context, columns []EncryptedColumn) (ReencryptStats, error) {
	targets := make([]rotationTarget, 0, len(columns))
	for _, col := range columns {
		targets = append(targets, rotationTarget{
			table: col.Table, column: col.Column, pkColumn: col.PKColumn, stage: "rotate",
			rotate: func(pk, value string) (string, bool, error) { return j.encryptor.RotateColumn(col.AAD(pk), value) },
			classify: func(pk, value string) (EncryptedPayloadClass, error) {
				return ClassifyEncryptedPayload(j.encryptor, col, pk, value)
			},
		})
	}
	return j.runTargets(ctx, targets)
}

// RunForSealedPayloads executes the job for non-column sealed payloads. The
// rotation primitive keeps the v1 generation: these values are read back with
// DecryptField, which cannot open a v2 (AAD-bound) token. Checkpoint, failure
// queue and statistics semantics are shared with RunFor.
func (j *ReencryptJob) RunForSealedPayloads(ctx context.Context, columns []SealedPayloadColumn) (ReencryptStats, error) {
	targets := make([]rotationTarget, 0, len(columns))
	for _, col := range columns {
		targets = append(targets, rotationTarget{
			table: col.Table, column: col.Column, pkColumn: col.PKColumn, stage: "reseal",
			rotate:   func(_, value string) (string, bool, error) { return j.encryptor.RotateSealedPayload(value) },
			classify: func(_, value string) (EncryptedPayloadClass, error) { return ClassifySealedPayload(j.encryptor, value) },
		})
	}
	return j.runTargets(ctx, targets)
}

// rotationTarget adapts the two registries (AAD-bound columns and v1 sealed
// payloads) to one resumable loop. The closure boundary is what keeps sealed
// payloads from ever receiving an AAD context.
type rotationTarget struct {
	table    string
	column   string
	pkColumn string
	stage    string // "rotate" (v2 columns) or "reseal" (v1 sealed payloads)
	rotate   func(pk, value string) (string, bool, error)
	classify func(pk, value string) (EncryptedPayloadClass, error)
}

func (j *ReencryptJob) runTargets(ctx context.Context, targets []rotationTarget) (ReencryptStats, error) {
	start := time.Now()
	j.resetStats()

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			break
		}
		j.mu.Lock()
		limitReached := j.maxRowsPerRun > 0 && j.stats.Scanned >= int64(j.maxRowsPerRun)
		j.mu.Unlock()
		if limitReached {
			break
		}
		if err := j.processTarget(ctx, target); err != nil {
			slog.Error("reencrypt: column failed", "table", target.table, "column", target.column, "err", err)
			j.mu.Lock()
			j.stats.Errors++
			j.mu.Unlock()
			// Continue with next column; checkpoint preserves per-column progress
		}
	}

	j.mu.Lock()
	j.stats.Duration = time.Since(start)
	j.stats.Completed = ctx.Err() == nil
	stats := j.stats
	j.mu.Unlock()

	// RunFor never clears the checkpoint: it may be called once per column, and
	// an early column's saved cursor must survive a later column's success. The
	// operator command clears the file only after every column finished without
	// errors (see ClearCheckpoint).
	return stats, nil
}

// ClearCheckpoint removes the resume file after a fully successful run.
func (j *ReencryptJob) ClearCheckpoint() error {
	if j.checkpoint == nil {
		return nil
	}
	return j.checkpoint.Clear()
}

func (j *ReencryptJob) resetStats() {
	j.mu.Lock()
	j.stats = ReencryptStats{}
	j.mu.Unlock()
}

// processTarget re-encrypts a single column or sealed payload in batches
// ordered by PK.
//
// scanCursor drives pagination within this run and always advances. resumeCursor
// is the checkpoint value and only advances while no row has failed: once a row
// fails, it stays at the last successful PK so the next run retries the failure
// instead of jumping over it. Rows already rewritten by this run are current and
// are skipped on the retry.
func (j *ReencryptJob) processTarget(ctx context.Context, target rotationTarget) error {
	scanCursor := j.loadTargetCursor(target)
	resumeCursor := scanCursor
	failed := false

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		rows, err := j.fetchTargetBatch(ctx, target.table, target.column, target.pkColumn, scanCursor)
		if err != nil {
			return apperrors.WrapInternal(err, "fetch batch")
		}

		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			scanCursor = row.pk
			j.mu.Lock()
			j.stats.Scanned++
			limitReached := j.maxRowsPerRun > 0 && j.stats.Scanned >= int64(j.maxRowsPerRun)
			j.mu.Unlock()
			if limitReached {
				// Online worker tick budget reached: persist the cursor and let the
				// next tick continue (never report the column as fully completed).
				j.saveTargetCursor(target, scanCursor)
				return nil
			}

			newValue, changed, err := target.rotate(row.pk, row.value)
			if err != nil {
				slog.Warn("reencrypt: row failed", "table", target.table, "column", target.column, "pk_fingerprint", pkFingerprint(row.pk), "err", err)
				j.recordRowFailure(target, row.pk, row.value, target.stage)
				failed = true
				continue
			}

			if !changed {
				j.mu.Lock()
				j.stats.Skipped++
				j.mu.Unlock()
				if !failed {
					resumeCursor = row.pk
				}
				continue
			}

			if err := j.updateTargetRow(ctx, target.table, target.column, target.pkColumn, row.pk, newValue); err != nil {
				slog.Warn("reencrypt: update failed", "table", target.table, "column", target.column, "pk_fingerprint", pkFingerprint(row.pk), "err", err)
				j.recordRowFailure(target, row.pk, row.value, "update")
				failed = true
				continue
			}

			j.mu.Lock()
			j.stats.Updated++
			j.mu.Unlock()
			if !failed {
				resumeCursor = row.pk
			}
		}

		if failed {
			j.saveTargetCursor(target, resumeCursor)
		} else {
			j.saveTargetCursor(target, scanCursor)
		}

		if len(rows) < j.batchSize {
			break
		}
	}

	return nil
}

func (j *ReencryptJob) recordRowFailure(target rotationTarget, pk, value, stage string) {
	class, _ := target.classify(pk, value)

	j.mu.Lock()
	defer j.mu.Unlock()
	j.stats.Errors++
	if j.maxFailures > 0 && len(j.stats.Failures) >= j.maxFailures {
		j.stats.FailuresTruncated = true
		return
	}
	j.stats.Failures = append(j.stats.Failures, EncryptedPayloadFailure{
		Table: target.table, Column: target.column, PK: pk, Class: class, Stage: stage,
	})
}

type fetchedRow struct {
	pk    string
	value string
}

func (j *ReencryptJob) fetchBatch(ctx context.Context, col EncryptedColumn, cursor string) ([]fetchedRow, error) {
	return j.fetchTargetBatch(ctx, col.Table, col.Column, col.PKColumn, cursor)
}

func (j *ReencryptJob) fetchTargetBatch(ctx context.Context, table, column, pkColumn, cursor string) ([]fetchedRow, error) {
	if err := validateIdentifiers(table, column, pkColumn); err != nil {
		return nil, err
	}
	// The first batch has no cursor. Omitting the predicate avoids comparing a
	// UUID PK against an empty string, which PostgreSQL rejects with
	// "invalid input syntax for type uuid".
	var (
		query string
		args  []any
	)
	if cursor == "" {
		// #nosec G201 -- identifiers validated above via validateIdentifiers().
		query = fmt.Sprintf(
			`SELECT %s, %s FROM %s ORDER BY %s ASC LIMIT %d`,
			pkColumn, column, table, pkColumn, j.batchSize,
		)
	} else {
		// #nosec G201 -- identifiers validated above via validateIdentifiers().
		query = fmt.Sprintf(
			`SELECT %s, %s FROM %s WHERE %s > $1 ORDER BY %s ASC LIMIT %d`,
			pkColumn, column, table, pkColumn, pkColumn, j.batchSize,
		)
		args = []any{cursor}
	}
	rows, err := j.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []fetchedRow
	for rows.Next() {
		var pk, val sql.NullString
		if err := rows.Scan(&pk, &val); err != nil {
			return nil, err
		}
		if !val.Valid || val.String == "" {
			continue
		}
		out = append(out, fetchedRow{pk: pk.String, value: val.String})
	}
	return out, rows.Err()
}

func (j *ReencryptJob) updateRow(ctx context.Context, col EncryptedColumn, pk, newValue string) error {
	return j.updateTargetRow(ctx, col.Table, col.Column, col.PKColumn, pk, newValue)
}

func (j *ReencryptJob) updateTargetRow(ctx context.Context, table, column, pkColumn, pk, newValue string) error {
	if err := validateIdentifiers(table, column, pkColumn); err != nil {
		return err
	}
	// #nosec G201 -- identifiers validated above via validateIdentifiers().
	query := fmt.Sprintf(`UPDATE %s SET %s = $1 WHERE %s = $2`, table, column, pkColumn)
	result, err := j.db.ExecContext(ctx, query, newValue, pk)
	if err != nil {
		return apperrors.WrapInternalf(err, "update %s.%s", table, column)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return apperrors.WrapInternalf(err, "rows affected %s.%s", table, column)
	}
	if affected == 0 {
		// The PK disappeared between the scan and the update. Report it as a
		// row error so the next run retries instead of silently skipping it.
		return apperrors.NewInternalf("%s.%s row %s vanished before re-encryption", table, column, pkFingerprint(pk))
	}
	return nil
}

// pkFingerprint returns a stable, non-reversible fingerprint for audit logs.
// ADR-015 forbids logging the primary key itself.
func pkFingerprint(pk string) string {
	sum := sha256.Sum256([]byte(pk))
	return hex.EncodeToString(sum[:8])
}

func (j *ReencryptJob) loadCursor(col EncryptedColumn) string {
	return j.loadTargetCursor(rotationTarget{table: col.Table, column: col.Column})
}

func (j *ReencryptJob) loadTargetCursor(target rotationTarget) string {
	if j.checkpoint == nil {
		return ""
	}
	return j.checkpoint.Load(target.table + "." + target.column)
}

func (j *ReencryptJob) saveCursor(col EncryptedColumn, cursor string) {
	j.saveTargetCursor(rotationTarget{table: col.Table, column: col.Column}, cursor)
}

func (j *ReencryptJob) saveTargetCursor(target rotationTarget, cursor string) {
	if j.checkpoint == nil {
		return
	}
	_ = j.checkpoint.Save(target.table+"."+target.column, cursor)
}

// Stats returns a snapshot of the current job statistics.
func (j *ReencryptJob) Stats() ReencryptStats {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.stats
}

// CheckpointStore persists re-encryption progress to a file so a job can be
// resumed after interruption. It stores a map of "table.column" -> last
// processed PK cursor.
type CheckpointStore struct {
	path string
	mu   sync.Mutex
}

// NewCheckpointStore creates a checkpoint store at the given file path.
func NewCheckpointStore(path string) *CheckpointStore {
	return &CheckpointStore{path: path}
}

// Load returns the saved cursor for the given key, or "" if none.
func (s *CheckpointStore) Load(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return parts[1]
		}
	}
	return ""
}

// Save persists the cursor for the given key.
func (s *CheckpointStore) Save(key, cursor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cursors := map[string]string{}
	if data, err := os.ReadFile(s.path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && parts[0] != "" {
				cursors[parts[0]] = parts[1]
			}
		}
	}
	cursors[key] = cursor

	lines := make([]string, 0, len(cursors))
	for k, v := range cursors {
		lines = append(lines, k+"="+v)
	}
	return os.WriteFile(s.path, []byte(strings.Join(lines, "\n")), 0o600)
}

// Clear removes the checkpoint file. Call after a successful full run.
func (s *CheckpointStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
