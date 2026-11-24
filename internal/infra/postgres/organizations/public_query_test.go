package organizations

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

const transactionLocalSuperAdminSQL = `SELECT set_config('app.current_role', 'super_admin', true)`

type fakePublicDiscoveryPool struct {
	conn publicDiscoveryConn
	err  error
}

func (p fakePublicDiscoveryPool) Acquire(context.Context) (publicDiscoveryConn, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.conn, nil
}

type fakePublicDiscoveryConn struct {
	tx       publicDiscoveryTx
	err      error
	releases int
}

func (c *fakePublicDiscoveryConn) Begin(context.Context) (publicDiscoveryTx, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.tx, nil
}

func (c *fakePublicDiscoveryConn) Release() { c.releases++ }

type fakePublicDiscoveryTx struct {
	execSQL   []string
	querySQL  []string
	execErr   error
	queryRow  pgx.Row
	commitErr error
	commits   int
	rollbacks int
}

func (t *fakePublicDiscoveryTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	t.execSQL = append(t.execSQL, sql)
	if t.execErr != nil {
		return pgconn.CommandTag{}, t.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (t *fakePublicDiscoveryTx) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	t.querySQL = append(t.querySQL, sql)
	return t.queryRow
}

func (t *fakePublicDiscoveryTx) Commit(context.Context) error {
	t.commits++
	return t.commitErr
}

func (t *fakePublicDiscoveryTx) Rollback(context.Context) error {
	t.rollbacks++
	return nil
}

func fakePublicRepository(tx *fakePublicDiscoveryTx) (Repository, *fakePublicDiscoveryConn) {
	conn := &fakePublicDiscoveryConn{tx: tx}
	return Repository{publicPool: fakePublicDiscoveryPool{conn: conn}}, conn
}

// TestGetBySlugPublic_UsesTransactionLocalSuperAdmin pins the GUC regression for
// the anonymous /by-slug discovery path: the role elevation must be
// transaction-local (set_config(..., true)), never session-level, and the query
// must select public columns only. Commit + Release prove the borrowed
// connection is returned without carrying the elevation to the next borrower.
func TestGetBySlugPublic_UsesTransactionLocalSuperAdmin(t *testing.T) {
	tx := &fakePublicDiscoveryTx{queryRow: shared.NewMockRow(make([]any, 19), nil)}
	repo, conn := fakePublicRepository(tx)

	organization, err := repo.GetBySlugPublic(context.Background(), "acme")
	require.NoError(t, err)
	require.NotNil(t, organization)

	require.Len(t, tx.execSQL, 1)
	assert.Equal(t, transactionLocalSuperAdminSQL, tx.execSQL[0])
	assert.NotContains(t, strings.Join(tx.execSQL, "\n"), "super_admin', false",
		"public lookup must not promote the role at session scope")
	assert.Equal(t, 1, tx.commits)
	assert.Zero(t, tx.rollbacks)
	assert.Equal(t, 1, conn.releases)

	require.Len(t, tx.querySQL, 1)
	assert.Equal(t, queryGetOrganizationBySlugPublic, tx.querySQL[0])
	for _, forbidden := range []string{
		"master_password",
		"master_verification_code",
		"default_password",
		"password_salt",
		"password_obfuscator_key",
		"kerberos_keytab",
	} {
		assert.NotContains(t, tx.querySQL[0], forbidden,
			"public query must not read credential column %q", forbidden)
	}
	for _, public := range []string{
		"o.id",
		"o.slug",
		"o.display_name",
		"o.website_url",
		"o.logo_url",
		"o.logo_dark_url",
		"o.favicon_url",
		"o.default_avatar",
		"o.status",
		"o.disable_signin",
		"o.initiate_login_url",
		"ac.password_type",
		"s.default_application_id",
		"t.theme_name",
		"t.tokens",
		"t.custom_css",
		"t.assets",
	} {
		assert.Contains(t, tx.querySQL[0], public,
			"public query must keep public column %q", public)
	}
}

// TestGetBySlugPublic_RollsBackOnQueryError pins that a failed lookup still
// closes the transaction, so the transaction-local super_admin cannot survive
// the request.
func TestGetBySlugPublic_RollsBackOnQueryError(t *testing.T) {
	tx := &fakePublicDiscoveryTx{
		queryRow: shared.NewMockRow(nil, errors.New("query failed")),
	}
	repo, conn := fakePublicRepository(tx)

	_, err := repo.GetBySlugPublic(context.Background(), "acme")
	require.Error(t, err)
	assert.Zero(t, tx.commits)
	assert.Equal(t, 1, tx.rollbacks)
	assert.Equal(t, 1, conn.releases)
}

// TestGetBySlugPublic_RollsBackOnSetConfigError pins the same cleanup when the
// elevation itself fails.
func TestGetBySlugPublic_RollsBackOnSetConfigError(t *testing.T) {
	tx := &fakePublicDiscoveryTx{execErr: errors.New("set_config failed")}
	repo, conn := fakePublicRepository(tx)

	_, err := repo.GetBySlugPublic(context.Background(), "acme")
	require.Error(t, err)
	assert.Zero(t, tx.commits)
	assert.Equal(t, 1, tx.rollbacks)
	assert.Equal(t, 1, conn.releases)
	assert.Empty(t, tx.querySQL, "no public query may run without the elevation")
}
