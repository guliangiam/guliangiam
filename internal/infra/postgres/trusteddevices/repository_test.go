package trusteddevices

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ shared.DBTX = (*recordingDBTX)(nil)

// recordingDBTX captures Exec calls so the persistence boundary can be asserted
// without a live database.
type recordingDBTX struct {
	execCalls int
	sql       string
	args      []any
	tag       pgconn.CommandTag
}

func (f *recordingDBTX) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	f.execCalls++
	f.sql = sql
	f.args = arguments
	return f.tag, nil
}

func (f *recordingDBTX) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (f *recordingDBTX) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func newDevice(credential string) *domain.TrustedDevice {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	return &domain.TrustedDevice{
		ID:                 "device-1",
		OrganizationID:     "org-1",
		UserID:             "user-1",
		DeviceCredentialID: credential,
		DevicePublicKey:    []byte("key"),
		UAHash:             "ua-hash",
		CreatedAt:          now,
		LastSeenAt:         now,
		ExpiresAt:          now.Add(30 * 24 * time.Hour),
		Revision:           1,
		UpdatedAt:          now,
	}
}

func TestCreateRejectsPlaintextCredentialBeforeSQL(t *testing.T) {
	db := &recordingDBTX{}
	repo := NewRepository(db)

	err := repo.Create(context.Background(), newDevice("plaintext-credential-0123456789abcdef"))
	if err == nil {
		t.Fatal("Create accepted a plaintext credential")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error is not *apperrors.Error: %T", err)
	}
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("code = %v, want UnprocessableEntity", appErr.Code)
	}
	if db.execCalls != 0 {
		t.Fatalf("Exec called %d times, want 0 (validation must run before SQL)", db.execCalls)
	}
}

func TestCreatePassesDigestAsCredentialArgument(t *testing.T) {
	db := &recordingDBTX{}
	repo := NewRepository(db)

	credential := digest("plaintext-credential")
	if err := repo.Create(context.Background(), newDevice(credential)); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if db.execCalls != 1 {
		t.Fatalf("Exec calls = %d, want 1", db.execCalls)
	}
	if db.sql != queryInsertDevice {
		t.Fatalf("Exec query = %q, want queryInsertDevice", db.sql)
	}
	// arguments: ID, OrganizationID, UserID, DeviceCredentialID, ...
	if len(db.args) < 4 {
		t.Fatalf("Exec args = %d, want >= 4", len(db.args))
	}
	if got := db.args[3]; got != credential {
		t.Fatalf("credential argument = %v, want digest %q", got, credential)
	}
}

func TestValidateCredentialDigest(t *testing.T) {
	if err := domain.ValidateCredentialDigest(digest("x")); err != nil {
		t.Fatalf("valid digest rejected: %v", err)
	}
	for _, invalid := range []string{
		"",
		"plaintext",
		"ABC123",
		digest("x")[:63],
		digest("x") + "0",
		"ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789",
		"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
	} {
		if err := domain.ValidateCredentialDigest(invalid); err == nil {
			t.Fatalf("ValidateCredentialDigest(%q) = nil, want error", invalid)
		}
	}
}

// TestRevokeByUserScopesToTenantAndReturnsAffectedRows 覆盖凭据级联的持久化边界：
// 租户与用户两个参数必须进入 WHERE，且返回真实受影响行数供审计计数。
func TestRevokeByUserScopesToTenantAndReturnsAffectedRows(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	db := &recordingDBTX{tag: pgconn.NewCommandTag("UPDATE 3")}
	repo := NewRepository(db)

	affected, err := repo.RevokeByUser(context.Background(), "org-1", "user-1", now)
	if err != nil {
		t.Fatalf("RevokeByUser() error = %v", err)
	}
	if affected != 3 {
		t.Fatalf("RowsAffected = %d, want 3", affected)
	}
	if db.sql != queryRevokeByUser {
		t.Fatalf("Exec query = %q, want queryRevokeByUser", db.sql)
	}
	if len(db.args) != 3 {
		t.Fatalf("Exec args = %d, want 3", len(db.args))
	}
	if db.args[0] != "org-1" || db.args[1] != "user-1" || db.args[2] != now {
		t.Fatalf("Exec args = %v, want [org-1 user-1 now]", db.args)
	}
	if !strings.Contains(db.sql, "revoked_at IS NULL") {
		t.Fatalf("query must only revoke still-active devices: %s", db.sql)
	}
}
