package organizations

import (
	"context"
	stderrors "errors"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// aggregateRow holds all columns from the JOIN query (organization + settings + theme + security).
type aggregateRow struct {
	base     organizationRow
	settings organizationSettingsRow
	hasSet   bool
	theme    organizationThemeRow
	hasTheme bool
	security organizationSecurityRow
	hasSec   bool
}

// organizationPublicRow is the scan target of queryGetOrganizationBySlugPublic.
// It mirrors only the columns that the anonymous discovery endpoints may expose.
type organizationPublicRow struct {
	ID                   string
	Slug                 string
	DisplayName          string
	WebsiteURL           string
	LogoURL              string
	LogoDarkURL          string
	FaviconURL           string
	DefaultAvatar        string
	Status               string
	DisableSignin        bool
	InitiateLoginURL     string
	PasswordType         string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DefaultApplicationID string
	ThemeName            string
	ThemeTokens          []byte
	ThemeCustomCSS       string
	ThemeAssets          []byte
}

// toDomain builds an organization aggregate that carries public fields only;
// credential fields keep their zero values and are never decrypted.
func (row organizationPublicRow) toDomain() (*domain.Organization, error) {
	theme := domain.OrganizationThemeSettings{
		OrganizationID: row.ID,
		ThemeName:      row.ThemeName,
		CustomCSS:      row.ThemeCustomCSS,
	}
	if err := shared.UnmarshalJSON(row.ThemeTokens, &theme.Tokens); err != nil {
		return nil, err
	}
	if err := shared.UnmarshalJSON(row.ThemeAssets, &theme.Assets); err != nil {
		return nil, err
	}

	return &domain.Organization{
		ID:               row.ID,
		Slug:             row.Slug,
		DisplayName:      row.DisplayName,
		WebsiteURL:       row.WebsiteURL,
		LogoURL:          row.LogoURL,
		LogoDarkURL:      row.LogoDarkURL,
		FaviconURL:       row.FaviconURL,
		Status:           row.Status,
		DefaultAvatar:    row.DefaultAvatar,
		DisableSignin:    row.DisableSignin,
		InitiateLoginURL: row.InitiateLoginURL,
		PasswordType:     row.PasswordType,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		Settings: domain.OrganizationSettings{
			OrganizationID:       row.ID,
			DefaultApplicationID: row.DefaultApplicationID,
		},
		Theme: theme,
	}, nil
}

type Repository struct {
	db         shared.DBTX
	publicPool publicDiscoveryPool // optional — set for pool-level RLS-bypass queries
}

// publicDiscoveryPool/publicDiscoveryConn/publicDiscoveryTx narrow the pgx
// surface used by GetBySlugPublic. Keeping the seam behind interfaces lets the
// transaction-local role elevation be pinned by a unit test without a real
// PostgreSQL connection.
type publicDiscoveryPool interface {
	Acquire(ctx context.Context) (publicDiscoveryConn, error)
}

type publicDiscoveryConn interface {
	Begin(ctx context.Context) (publicDiscoveryTx, error)
	Release()
}

type publicDiscoveryTx interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type pgxPublicDiscoveryPool struct {
	pool *pgxpool.Pool
}

func (p pgxPublicDiscoveryPool) Acquire(ctx context.Context) (publicDiscoveryConn, error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	return pgxPublicDiscoveryConn{conn: conn}, nil
}

type pgxPublicDiscoveryConn struct {
	conn *pgxpool.Conn
}

func (c pgxPublicDiscoveryConn) Begin(ctx context.Context) (publicDiscoveryTx, error) {
	tx, err := c.conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return pgxPublicDiscoveryTx{tx: tx}, nil
}

func (c pgxPublicDiscoveryConn) Release() { c.conn.Release() }

type pgxPublicDiscoveryTx struct {
	tx pgx.Tx
}

func (t pgxPublicDiscoveryTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return t.tx.Exec(ctx, sql, arguments...)
}

func (t pgxPublicDiscoveryTx) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	return t.tx.QueryRow(ctx, sql, arguments...)
}

func (t pgxPublicDiscoveryTx) Commit(ctx context.Context) error { return t.tx.Commit(ctx) }

func (t pgxPublicDiscoveryTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

// NewRepository constructs a Repository bound to db. An optional *pgxpool.Pool
// may be passed for pool-level RLS bypass (e.g. public by-slug lookup).
// Variadic so existing call sites (UOW) continue to compile unchanged.
func NewRepository(db shared.DBTX, pools ...*pgxpool.Pool) Repository {
	r := Repository{db: db}
	if len(pools) > 0 && pools[0] != nil {
		r.publicPool = pgxPublicDiscoveryPool{pool: pools[0]}
	}
	return r
}

// beginner is implemented by both pgx.Pool and pgx.Tx (which uses savepoints).
// This allows withinTransaction to work whether r.db is a pool connection or
// an already-active transaction.
type beginner interface {
	shared.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// withinTransaction executes fn inside a database transaction. If r.db implements
// beginner (i.e. is a pgx.Pool or pgx.Tx), a new transaction or savepoint is
// opened; otherwise fn runs directly so test mocks that only implement DBTX keep working.
func (r Repository) withinTransaction(ctx context.Context, fn func(ctx context.Context, tx shared.DBTX) error) error {
	if b, ok := r.db.(beginner); ok {
		tx, err := b.Begin(ctx)
		if err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback(ctx)
			}
		}()

		if err := fn(ctx, tx); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}
		committed = true
		return nil
	}

	// r.db does not support Begin (e.g. test mock); execute directly.
	return fn(ctx, r.db)
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	agg, err := r.getAggregateBy(ctx, queryGetOrganizationByID, id)
	if err != nil {
		return nil, err
	}

	return r.loadAggregate(agg)
}

func (r Repository) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	agg, err := r.getAggregateBy(ctx, queryGetOrganizationBySlug, slug)
	if err != nil {
		return nil, err
	}

	return r.loadAggregate(agg)
}

// GetSessionPolicy implements domain.SessionPolicyReader. It is the narrow read
// used while resolving a session: one column from one row, no secret decryption
// and no settings/theme joins.
func (r Repository) GetSessionPolicy(ctx context.Context, organizationID string) (map[string]any, error) {
	var raw []byte
	if err := r.db.QueryRow(ctx, queryGetOrganizationSessionPolicy, organizationID).Scan(&raw); err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("organization not found")
		}
		return nil, err
	}

	policy := map[string]any{}
	if err := shared.UnmarshalJSON(raw, &policy); err != nil {
		return nil, err
	}

	return policy, nil
}

// GetBySlugPublic serves the anonymous /by-slug and /resolve discovery endpoints.
//
// RLS necessity: guliangiam.organizations uses FORCE ROW LEVEL SECURITY with
// USING (is_super_admin() OR id = current_organization_id()). An anonymous
// request carries no tenant GUC, so the row filter would hide the target row;
// a temporary super_admin role is therefore still required. To keep that
// elevation as narrow as possible:
//   - set_config(..., true) is transaction-local, so COMMIT/ROLLBACK restores
//     the pooled connection and super_admin never leaks to the next borrower;
//   - the query selects public columns only (branding, theme, default
//     application) and never reads master_password, master_verification_code,
//     default_password, password_salt, password_obfuscator_key or
//     kerberos_keytab;
//   - loadAggregate/decryptOrganizationSecrets are not used on this path.
func (r Repository) GetBySlugPublic(ctx context.Context, slug string) (*domain.Organization, error) {
	if r.publicPool == nil {
		// No pool available — fall back to normal (RLS-protected) path.
		return r.GetBySlug(ctx, slug)
	}

	conn, err := r.publicPool.Acquire(ctx)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "acquire connection for public org lookup")
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "begin public org lookup transaction")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_role', 'super_admin', true)`); err != nil {
		return nil, apperrors.WrapInternalf(err, "set super_admin for public org lookup")
	}

	row, err := scanOrganizationPublicRow(tx.QueryRow(ctx, queryGetOrganizationBySlugPublic, slug))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("organization not found")
		}
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, apperrors.WrapInternalf(err, "commit public org lookup transaction")
	}
	committed = true

	return row.toDomain()
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Organization, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListOrganizations, query.Keyword, query.Status, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Organization, 0)
	for rows.Next() {
		row, err := scanOrganizationRow(rows)
		if err != nil {
			return nil, 0, err
		}
		item := toOrganization(row)
		if err := decryptOrganizationSecrets(item); err != nil {
			return nil, 0, err
		}
		// List rows carry only the settings column the list projection needs;
		// without this the API returns defaultApplicationId as "" for every row.
		item.Settings.DefaultApplicationID = row.DefaultApplicationID
		items = append(items, item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountOrganizations, query.Keyword, query.Status).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, organization *domain.Organization) error {
	masterPassword, masterVerificationCode, defaultPassword, err := organizationSecretsForWrite(organization)
	if err != nil {
		return err
	}

	// 主表与拆分出的子表必须同一事务写入（ADR-022 God Table 拆分）。
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		if _, err := tx.Exec(ctx, queryInsertOrganizationCore,
			organization.ID, organization.Slug, organization.DisplayName, organization.WebsiteURL, organization.LogoURL,
			organization.LogoDarkURL, organization.FaviconURL, organization.Status, organization.InitScore, organization.DefaultAvatar,
			organization.DisableSignin, organization.EnableSoftDeletion, organization.DisableSignup,
			organization.IpWhitelist, organization.InitiateLoginURL, organization.CreatedAt, organization.UpdatedAt, organization.Revision, organization.ParentID, organization.RegionCode); err != nil {
			// 并发下 slug 唯一约束冲突 → 409（DEF-33），不透传原始 pg 错误（500）。
			var pgErr *pgconn.PgError
			if stderrors.As(err, &pgErr) && pgErr.Code == "23505" {
				return apperrors.NewConflict("organization slug already exists")
			}
			return err
		}

		return r.upsertOrganizationSubtables(ctx, tx, organization, masterPassword, masterVerificationCode, defaultPassword)
	})
}

func (r Repository) Update(ctx context.Context, organization *domain.Organization) error {
	masterPassword, masterVerificationCode, defaultPassword, err := organizationSecretsForWrite(organization)
	if err != nil {
		return err
	}

	// 乐观锁守卫与子表写入必须同一事务：主表 revision 不匹配时子表不得被改写。
	return r.withinTransaction(ctx, func(ctx context.Context, tx shared.DBTX) error {
		var newRevision int64
		err := tx.QueryRow(ctx, queryUpdateOrganizationCore,
			organization.DisplayName, organization.WebsiteURL, organization.LogoURL,
			organization.LogoDarkURL, organization.FaviconURL, organization.Status,
			organization.InitScore, organization.DefaultAvatar, organization.DisableSignin,
			organization.EnableSoftDeletion, organization.DisableSignup, organization.IpWhitelist,
			organization.InitiateLoginURL, organization.ParentID, organization.UpdatedAt, organization.RegionCode,
			organization.Revision-1, organization.ID).Scan(&newRevision)
		if err != nil {
			if err == pgx.ErrNoRows {
				return apperrors.NewConflict("organization revision conflict")
			}
			return err
		}

		if err := r.upsertOrganizationSubtables(ctx, tx, organization, masterPassword, masterVerificationCode, defaultPassword); err != nil {
			return err
		}

		organization.Revision = newRevision
		return nil
	})
}

// upsertOrganizationSubtables 写入 ADR-022 God Table 拆分出的 4 张子表。每条语句都是
// 以 organization_id 为键的 upsert，使创建与更新共用一条路径；事务由调用方持有。
func (r Repository) upsertOrganizationSubtables(ctx context.Context, tx shared.DBTX, organization *domain.Organization, masterPassword, masterVerificationCode, defaultPassword string) error {
	// password_options / ldap_attributes 是 NOT NULL 的 varchar[] 列。pgx 把 nil 切片
	// 编码成 SQL NULL，而显式 NULL 不会触发列默认值 '{}'，会直接 23502。这条 upsert
	// 是整行覆盖语义，nil 切片与空切片都表示「没有条目」，因此统一归一为空数组。
	if _, err := tx.Exec(ctx, queryUpsertOrganizationAuthConfig, organization.ID, organization.PasswordObfuscatorType, nonNilStrings(organization.PasswordOptions), organization.PasswordType, organization.PasswordSalt, organization.PasswordObfuscatorKey, masterPassword, masterVerificationCode, defaultPassword, organization.UseEmailAsUsername, nonNilStrings(organization.LdapAttributes)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, queryUpsertOrganizationKerberosConfig, organization.ID, organization.KerberosRealm, organization.KerberosKdcHost, organization.KerberosKeytab, organization.KerberosServiceName); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, queryUpsertOrganizationBilling, organization.ID, organization.Balance, organization.Currency, organization.EnableDeposit, organization.BalanceCredit, organization.Mu, organization.DefaultQuota, organization.DefaultCurrency); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, queryUpsertOrganizationBranding, organization.ID, organization.EnableDarkLogo, organization.HasPrivilegeConsent, organization.IsProfilePublic, organization.UsePermanentAvatar, organization.EnableTour); err != nil {
		return err
	}
	return nil
}

func (r Repository) UpdateSettings(ctx context.Context, settings *domain.OrganizationSettings) error {
	languages, err := shared.MarshalJSON(settings.Languages)
	if err != nil {
		return err
	}
	accountItems, err := shared.MarshalJSON(settings.AccountItems)
	if err != nil {
		return err
	}
	navPreferences, err := shared.MarshalJSON(settings.NavPreferences)
	if err != nil {
		return err
	}
	featureFlags, err := shared.MarshalJSON(settings.FeatureFlags)
	if err != nil {
		return err
	}
	countryCodes, err := shared.MarshalJSON(settings.CountryCodes)
	if err != nil {
		return err
	}
	userTypes, err := shared.MarshalJSON(settings.UserTypes)
	if err != nil {
		return err
	}
	tags, err := shared.MarshalJSON(settings.Tags)
	if err != nil {
		return err
	}
	mfaItems, err := shared.MarshalJSON(settings.MfaItems)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryUpsertOrganizationSettings, settings.OrganizationID, languages, settings.DefaultApplicationID, accountItems, navPreferences, featureFlags, countryCodes, userTypes, tags, mfaItems, settings.AccountMenu, settings.MfaRememberInHours, settings.DcrPolicy)
	return err
}

func (r Repository) UpdateTheme(ctx context.Context, theme *domain.OrganizationThemeSettings) error {
	tokens, err := shared.MarshalJSON(theme.Tokens)
	if err != nil {
		return err
	}
	assets, err := shared.MarshalJSON(theme.Assets)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryUpsertOrganizationTheme, theme.OrganizationID, theme.ThemeName, tokens, theme.CustomCSS, assets)
	return err
}

func (r Repository) UpdateSecurityPolicy(ctx context.Context, policy *domain.OrganizationSecurityPolicy) error {
	passwordPolicy, err := shared.MarshalJSON(policy.PasswordPolicy)
	if err != nil {
		return err
	}
	mfaPolicy, err := shared.MarshalJSON(policy.MfaPolicy)
	if err != nil {
		return err
	}
	ipPolicy, err := shared.MarshalJSON(policy.IPPolicy)
	if err != nil {
		return err
	}
	sessionPolicy, err := shared.MarshalJSON(policy.SessionPolicy)
	if err != nil {
		return err
	}
	riskPolicy, err := shared.MarshalJSON(policy.RiskPolicy)
	if err != nil {
		return err
	}
	lockoutPolicy, err := shared.MarshalJSON(policy.LockoutPolicy)
	if err != nil {
		return err
	}
	concurrentSessionPolicy, err := shared.MarshalJSON(policy.ConcurrentSessionPolicy)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryUpsertOrganizationSecurityPolicy, policy.OrganizationID, passwordPolicy, policy.EffectivePasswordDeriveAlgorithm(), mfaPolicy, ipPolicy, sessionPolicy, riskPolicy, lockoutPolicy, concurrentSessionPolicy)
	return err
}

// scannable is satisfied by both pgx.Row and pgx.Rows.
type scannable interface {
	Scan(dest ...any) error
}

// scanOrganizationRow scans all columns from an organization SELECT query (base table only).
// Column order must match the SELECT column order in sql.go (queryListOrganizations).
func scanOrganizationRow(s scannable) (organizationRow, error) {
	var row organizationRow
	err := s.Scan(
		&row.ID, &row.Slug, &row.DisplayName, &row.WebsiteURL, &row.LogoURL,
		&row.LogoDarkURL, &row.FaviconURL, &row.Status, &row.InitScore, &row.DefaultAvatar,
		&row.PasswordObfuscatorType, &row.DisableSignin, &row.PasswordOptions, &row.EnableSoftDeletion, &row.DisableSignup,
		&row.IpWhitelist, &row.InitiateLoginURL, &row.Balance, &row.Currency,
		// Phase 2 extended fields
		&row.PasswordType, &row.PasswordSalt, &row.PasswordObfuscatorKey, &row.EnableDeposit, &row.BalanceCredit,
		&row.Mu, &row.DefaultQuota, &row.DefaultCurrency, &row.EnableDarkLogo, &row.HasPrivilegeConsent,
		&row.IsProfilePublic, &row.UseEmailAsUsername, &row.UsePermanentAvatar, &row.EnableTour, &row.MasterPassword,
		&row.MasterVerificationCode, &row.DefaultPassword, &row.LdapAttributes, &row.KerberosRealm, &row.KerberosKdcHost, &row.KerberosKeytab,
		&row.KerberosServiceName,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		&row.DefaultApplicationID,
		&row.ParentID,
		&row.RegionCode,
	)
	return row, err
}

// scanOrganizationPublicRow scans the public-column projection used by
// queryGetOrganizationBySlugPublic (19 columns, see sql.go).
func scanOrganizationPublicRow(s scannable) (organizationPublicRow, error) {
	var row organizationPublicRow
	err := s.Scan(
		&row.ID, &row.Slug, &row.DisplayName, &row.WebsiteURL, &row.LogoURL, &row.LogoDarkURL, &row.FaviconURL,
		&row.DefaultAvatar, &row.Status, &row.DisableSignin, &row.InitiateLoginURL, &row.PasswordType,
		&row.CreatedAt, &row.UpdatedAt,
		&row.DefaultApplicationID,
		&row.ThemeName, &row.ThemeTokens, &row.ThemeCustomCSS, &row.ThemeAssets,
	)
	return row, err
}

// scanAggregateRow scans all columns from the organization JOIN query.
// Column order must match the SELECT column order in sql.go:
//   - base organization columns (44 fields)
//   - settings columns (13 fields, nullable via LEFT JOIN)
//   - theme columns (5 fields, nullable via LEFT JOIN)
//   - security columns (8 fields, nullable via LEFT JOIN)
func scanAggregateRow(s scannable) (aggregateRow, error) {
	var row aggregateRow

	// Prepare nullable containers for LEFT JOIN columns.
	// Settings
	var setOrgID *string
	var setLang, setAcctItems, setNavPref, setFeatFlags, setCountryCodes, setUserTypes, setTags, setMfaItems *[]byte
	var setDefAppID *string
	var setAcctMenu *string
	var setMfaHours *int
	var setDcrPolicy *string

	// Theme
	var themeOrgID *string
	var themeName *string
	var themeTokens, themeAssets *[]byte
	var themeCSS *string

	// Security
	var secOrgID *string
	var secPwdDeriveAlg *string
	var secPwdPolicy, secMfaPolicy, secIpPolicy, secSessPolicy, secRiskPolicy, secLockoutPolicy, secConcSessPolicy *[]byte

	err := s.Scan(
		// Base organization columns
		&row.base.ID, &row.base.Slug, &row.base.DisplayName, &row.base.WebsiteURL, &row.base.LogoURL,
		&row.base.LogoDarkURL, &row.base.FaviconURL, &row.base.Status, &row.base.InitScore, &row.base.DefaultAvatar,
		&row.base.PasswordObfuscatorType, &row.base.DisableSignin, &row.base.PasswordOptions, &row.base.EnableSoftDeletion, &row.base.DisableSignup,
		&row.base.IpWhitelist, &row.base.InitiateLoginURL, &row.base.Balance, &row.base.Currency,
		&row.base.PasswordType, &row.base.PasswordSalt, &row.base.PasswordObfuscatorKey, &row.base.EnableDeposit, &row.base.BalanceCredit,
		&row.base.Mu, &row.base.DefaultQuota, &row.base.DefaultCurrency, &row.base.EnableDarkLogo, &row.base.HasPrivilegeConsent,
		&row.base.IsProfilePublic, &row.base.UseEmailAsUsername, &row.base.UsePermanentAvatar, &row.base.EnableTour, &row.base.MasterPassword,
		&row.base.MasterVerificationCode, &row.base.DefaultPassword, &row.base.LdapAttributes, &row.base.KerberosRealm, &row.base.KerberosKdcHost, &row.base.KerberosKeytab,
		&row.base.KerberosServiceName,
		&row.base.CreatedAt, &row.base.UpdatedAt, &row.base.Revision,
		// Settings columns (nullable)
		&setOrgID, &setLang, &setDefAppID, &setAcctItems, &setNavPref, &setFeatFlags, &setCountryCodes, &setUserTypes, &setTags, &setMfaItems, &setAcctMenu, &setMfaHours, &setDcrPolicy,
		// Theme columns (nullable)
		&themeOrgID, &themeName, &themeTokens, &themeCSS, &themeAssets,
		// Security columns (nullable)
		&secOrgID, &secPwdPolicy, &secPwdDeriveAlg, &secMfaPolicy, &secIpPolicy, &secSessPolicy, &secRiskPolicy, &secLockoutPolicy, &secConcSessPolicy,
		&row.base.ParentID,
		&row.base.RegionCode,
	)
	if err != nil {
		return row, err
	}

	// Materialize settings if present
	if setOrgID != nil {
		row.hasSet = true
		row.settings.OrganizationID = *setOrgID
		row.settings.Languages = derefBytes(setLang)
		row.settings.DefaultApplicationID = derefString(setDefAppID)
		row.settings.AccountItems = derefBytes(setAcctItems)
		row.settings.NavPreferences = derefBytes(setNavPref)
		row.settings.FeatureFlags = derefBytes(setFeatFlags)
		row.settings.CountryCodes = derefBytes(setCountryCodes)
		row.settings.UserTypes = derefBytes(setUserTypes)
		row.settings.Tags = derefBytes(setTags)
		row.settings.MfaItems = derefBytes(setMfaItems)
		row.settings.AccountMenu = derefString(setAcctMenu)
		row.settings.MfaRememberInHours = derefInt(setMfaHours)
		row.settings.DcrPolicy = derefString(setDcrPolicy)
	}

	// Materialize theme if present
	if themeOrgID != nil {
		row.hasTheme = true
		row.theme.OrganizationID = *themeOrgID
		row.theme.ThemeName = derefString(themeName)
		row.theme.Tokens = derefBytes(themeTokens)
		row.theme.CustomCSS = derefString(themeCSS)
		row.theme.Assets = derefBytes(themeAssets)
	}

	// Materialize security if present
	if secOrgID != nil {
		row.hasSec = true
		row.security.OrganizationID = *secOrgID
		row.security.PasswordDeriveAlgorithm = derefString(secPwdDeriveAlg)
		row.security.PasswordPolicy = derefBytes(secPwdPolicy)
		row.security.MfaPolicy = derefBytes(secMfaPolicy)
		row.security.IPPolicy = derefBytes(secIpPolicy)
		row.security.SessionPolicy = derefBytes(secSessPolicy)
		row.security.RiskPolicy = derefBytes(secRiskPolicy)
		row.security.LockoutPolicy = derefBytes(secLockoutPolicy)
		row.security.ConcurrentSessionPolicy = derefBytes(secConcSessPolicy)
	}

	return row, nil
}

// derefInt returns the dereferenced int or 0 if nil.
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// derefBytes returns the dereferenced byte slice or nil if nil.
func derefBytes(p *[]byte) []byte {
	if p == nil {
		return nil
	}
	return *p
}

func (r Repository) getAggregateBy(ctx context.Context, sql, value string) (aggregateRow, error) {
	row, err := scanAggregateRow(r.db.QueryRow(ctx, sql, value))
	if err != nil {
		if err == pgx.ErrNoRows {
			return aggregateRow{}, apperrors.NewNotFound("organization not found")
		}
		return aggregateRow{}, err
	}

	return row, nil
}

func (r Repository) loadAggregate(agg aggregateRow) (*domain.Organization, error) {
	organization := toOrganization(agg.base)
	if err := decryptOrganizationSecrets(organization); err != nil {
		return nil, err
	}

	if agg.hasSet {
		if err := applySettings(organization, agg.settings); err != nil {
			return nil, err
		}
	}
	if agg.hasTheme {
		if err := applyTheme(organization, agg.theme); err != nil {
			return nil, err
		}
	}
	if agg.hasSec {
		if err := applySecurity(organization, agg.security); err != nil {
			return nil, err
		}
	}

	return organization, nil
}

func (r Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, queryDeleteOrganization, id)
	return err
}

func (r Repository) HasDeletedWithSlug(ctx context.Context, slug string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, queryHasDeletedWithSlug, slug).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

var _ domain.Repository = Repository{}

// nonNilStrings collapses a nil slice to an empty slice. pgx encodes a nil
// []string as SQL NULL, and an explicit NULL bypasses a column default, so the
// NOT NULL array columns of organization_auth_config would reject it (23502).
func nonNilStrings(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}

// maxInt returns value when positive, otherwise fallback. Kept package-local
// (and renamed from max) so it does not shadow the predeclared max builtin.
func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
