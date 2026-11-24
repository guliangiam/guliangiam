package organizations

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 organizations 存储层。
// 与 internal/infra/postgres/organizations.Repository 实现同域接口。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// rowsAffected 从 sql.Result 安全提取影响行数，替代 PG 的 commandTag.RowsAffected()。
func rowsAffected(r sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

type beginner interface {
	mysqlshared.DBTX
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// withinTransaction 在 *sql.DB 上开启事务；已经处于 *sql.Tx 时复用外层事务。
func (r Repository) withinTransaction(ctx context.Context, fn func(ctx context.Context, tx mysqlshared.DBTX) error) error {
	if b, ok := r.db.(beginner); ok {
		tx, err := b.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()
		if err := fn(ctx, tx); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return nil
	}
	return fn(ctx, r.db)
}

// scannable 是 sql.Row 和 sql.Rows 的公共接口。
type scannable interface {
	Scan(dest ...any) error
}

// scanAggregateRow 扫描完整 JOIN 查询（GetByID / GetBySlug / List 用）。
// 列顺序必须与 sql.go queryGetOrganizationByID/Slug 严格一致：
//   - base organization (44 列)
//   - settings (13 列，LEFT JOIN 可 NULL，用指针承接)
//   - theme (5 列，LEFT JOIN 可 NULL)
//   - security (8 列，LEFT JOIN 可 NULL)
func scanAggregateRow(s scannable) (aggregateRow, error) {
	var row aggregateRow

	// Settings nullable holders
	var setOrgID *string
	var setLang, setAcctItems, setNavPref, setFeatFlags, setCountryCodes, setUserTypes, setTags, setMfaItems *[]byte
	var setDefAppID *string
	var setAcctMenu *string
	var setMfaHours *int
	var setDcrPolicy *string

	// Theme nullable holders
	var themeOrgID *string
	var themeName *string
	var themeTokens, themeAssets *[]byte
	var themeCSS *string

	// Security nullable holders
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

	if themeOrgID != nil {
		row.hasTheme = true
		row.theme.OrganizationID = *themeOrgID
		row.theme.ThemeName = derefString(themeName)
		row.theme.Tokens = derefBytes(themeTokens)
		row.theme.CustomCSS = derefString(themeCSS)
		row.theme.Assets = derefBytes(themeAssets)
	}

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

// organizationPublicRow 是 queryGetOrganizationBySlugPublic 的扫描目标，
// 只承载匿名发现端点可公开的列。
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

// scanOrganizationPublicRow 扫描公开列表投影（19 列，见 sql.go）。
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

// toDomain 组装只含公开字段的 organization；凭据字段保持零值且不会被解密。
func (row organizationPublicRow) toDomain() (*domain.Organization, error) {
	theme := domain.OrganizationThemeSettings{
		OrganizationID: row.ID,
		ThemeName:      row.ThemeName,
		CustomCSS:      row.ThemeCustomCSS,
	}
	if err := mysqlshared.UnmarshalJSON(row.ThemeTokens, &theme.Tokens); err != nil {
		return nil, err
	}
	if err := mysqlshared.UnmarshalJSON(row.ThemeAssets, &theme.Assets); err != nil {
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

// getAggregateBy 执行 JOIN 查询，返回 aggregateRow。
func (r Repository) getAggregateBy(ctx context.Context, query, value string) (aggregateRow, error) {
	row, err := scanAggregateRow(r.db.QueryRowContext(ctx, query, value))
	if err != nil {
		if err == sql.ErrNoRows {
			return aggregateRow{}, apperrors.NewNotFound("organization not found")
		}
		return aggregateRow{}, err
	}
	return row, nil
}

// loadAggregate 把 aggregateRow 组装成 domain.Organization。
func (r Repository) loadAggregate(agg aggregateRow) (*domain.Organization, error) {
	organization, err := toOrganization(agg.base)
	if err != nil {
		return nil, err
	}
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

// ── domain.Repository 实现 ────────────────────────────────────────

// GetByID 按主键取 organization（含 settings/theme/security JOIN）。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	agg, err := r.getAggregateBy(ctx, queryGetOrganizationByID, id)
	if err != nil {
		return nil, err
	}
	return r.loadAggregate(agg)
}

// GetBySlug 按 slug 取 organization（含 settings/theme/security JOIN）。
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
	if err := r.db.QueryRowContext(ctx, queryGetOrganizationSessionPolicy, organizationID).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("organization not found")
		}
		return nil, err
	}

	policy := map[string]any{}
	if err := mysqlshared.UnmarshalJSON(raw, &policy); err != nil {
		return nil, err
	}

	return policy, nil
}

// GetBySlugPublic MySQL 无 FORCE RLS，无需角色提升；但公共发现路径仍只读取
// 公开列（品牌、主题、默认应用），master_password / master_verification_code /
// default_password / password_salt / password_obfuscator_key / kerberos_keytab
// 不进入查询，避免未认证调用把凭据读进进程内存。
func (r Repository) GetBySlugPublic(ctx context.Context, slug string) (*domain.Organization, error) {
	row, err := scanOrganizationPublicRow(r.db.QueryRowContext(ctx, queryGetOrganizationBySlugPublic, slug))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("organization not found")
		}
		return nil, err
	}
	return row.toDomain()
}

// List 分页返回完整组织聚合，包含配置子表及 settings/theme/security。
// keyword 在 SQL 中出现 3 次，status 出现 2 次，需要分别绑定。
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Organization, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	// List: keyword × 3, status × 2, limit, offset = 7
	rows, err := r.db.QueryContext(ctx, queryListOrganizations,
		query.Keyword, query.Keyword, query.Keyword,
		query.Status, query.Status,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Organization, 0)
	for rows.Next() {
		row, err := scanAggregateRow(rows)
		if err != nil {
			return nil, 0, err
		}
		item, err := r.loadAggregate(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Count: keyword × 3, status × 2 = 5
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountOrganizations,
		query.Keyword, query.Keyword, query.Keyword,
		query.Status, query.Status,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Create 在同一事务中写入组织主表与四张配置子表。
func (r Repository) Create(ctx context.Context, organization *domain.Organization) error {
	masterPassword, masterVerificationCode, defaultPassword, err := organizationSecretsForWrite(organization)
	if err != nil {
		return err
	}
	passwordOptionsJSON, err := marshalPasswordOptions(organization.PasswordOptions)
	if err != nil {
		return err
	}
	ldapAttributesJSON, err := marshalLdapAttributes(organization.LdapAttributes)
	if err != nil {
		return err
	}

	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		if _, err := tx.ExecContext(ctx, queryInsertOrganization,
			organization.ID, organization.Slug, organization.DisplayName, organization.WebsiteURL, organization.LogoURL,
			organization.LogoDarkURL, organization.FaviconURL, organization.Status, organization.InitScore, organization.DefaultAvatar,
			organization.DisableSignin, organization.EnableSoftDeletion, organization.DisableSignup,
			organization.IpWhitelist, organization.InitiateLoginURL, organization.ParentID,
			organization.CreatedAt, organization.UpdatedAt, organization.Revision, organization.RegionCode,
		); err != nil {
			return err
		}
		return upsertOrganizationSubtables(ctx, tx, organization, masterPassword, masterVerificationCode, defaultPassword, passwordOptionsJSON, ldapAttributesJSON)
	})
}

// Update 在同一事务中执行乐观锁主表更新和配置子表 upsert。
func (r Repository) Update(ctx context.Context, organization *domain.Organization) error {
	masterPassword, masterVerificationCode, defaultPassword, err := organizationSecretsForWrite(organization)
	if err != nil {
		return err
	}
	passwordOptionsJSON, err := marshalPasswordOptions(organization.PasswordOptions)
	if err != nil {
		return err
	}
	ldapAttributesJSON, err := marshalLdapAttributes(organization.LdapAttributes)
	if err != nil {
		return err
	}

	return r.withinTransaction(ctx, func(ctx context.Context, tx mysqlshared.DBTX) error {
		res, err := tx.ExecContext(ctx, queryUpdateOrganization,
			organization.DisplayName, organization.WebsiteURL, organization.LogoURL,
			organization.LogoDarkURL, organization.FaviconURL, organization.Status,
			organization.InitScore, organization.DefaultAvatar, organization.DisableSignin,
			organization.EnableSoftDeletion, organization.DisableSignup, organization.IpWhitelist,
			organization.InitiateLoginURL, organization.ParentID, organization.UpdatedAt, organization.RegionCode,
			organization.ID, organization.Revision-1,
		)
		n, err := rowsAffected(res, err)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperrors.NewConflict("organization revision conflict")
		}

		if err := upsertOrganizationSubtables(ctx, tx, organization, masterPassword, masterVerificationCode, defaultPassword, passwordOptionsJSON, ldapAttributesJSON); err != nil {
			return err
		}

		var newRevision int64
		if err := tx.QueryRowContext(ctx, querySelectRevision, organization.ID).Scan(&newRevision); err != nil {
			if err == sql.ErrNoRows {
				return apperrors.NewNotFound("organization not found")
			}
			return err
		}
		organization.Revision = newRevision
		return nil
	})
}

func upsertOrganizationSubtables(
	ctx context.Context,
	tx mysqlshared.DBTX,
	organization *domain.Organization,
	masterPassword, masterVerificationCode, defaultPassword string,
	passwordOptionsJSON, ldapAttributesJSON []byte,
) error {
	if _, err := tx.ExecContext(ctx, queryUpsertOrganizationAuthConfig,
		organization.ID, organization.PasswordObfuscatorType, passwordOptionsJSON,
		organization.PasswordType, organization.PasswordSalt, organization.PasswordObfuscatorKey,
		masterPassword, masterVerificationCode, defaultPassword,
		organization.UseEmailAsUsername, ldapAttributesJSON,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertOrganizationKerberosConfig,
		organization.ID, organization.KerberosRealm, organization.KerberosKdcHost,
		organization.KerberosKeytab, organization.KerberosServiceName,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, queryUpsertOrganizationBilling,
		organization.ID, organization.Balance, organization.Currency, organization.EnableDeposit,
		organization.BalanceCredit, organization.Mu, organization.DefaultQuota, organization.DefaultCurrency,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, queryUpsertOrganizationBranding,
		organization.ID, organization.EnableDarkLogo, organization.HasPrivilegeConsent,
		organization.IsProfilePublic, organization.UsePermanentAvatar, organization.EnableTour,
	)
	return err
}

// UpdateSettings upsert organization_settings。
func (r Repository) UpdateSettings(ctx context.Context, settings *domain.OrganizationSettings) error {
	languages, err := mysqlshared.MarshalJSON(settings.Languages)
	if err != nil {
		return err
	}
	accountItems, err := mysqlshared.MarshalJSON(settings.AccountItems)
	if err != nil {
		return err
	}
	navPreferences, err := mysqlshared.MarshalJSON(settings.NavPreferences)
	if err != nil {
		return err
	}
	featureFlags, err := mysqlshared.MarshalJSON(settings.FeatureFlags)
	if err != nil {
		return err
	}
	countryCodes, err := mysqlshared.MarshalJSON(settings.CountryCodes)
	if err != nil {
		return err
	}
	userTypes, err := mysqlshared.MarshalJSON(settings.UserTypes)
	if err != nil {
		return err
	}
	tags, err := mysqlshared.MarshalJSON(settings.Tags)
	if err != nil {
		return err
	}
	mfaItems, err := mysqlshared.MarshalJSON(settings.MfaItems)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryUpsertOrganizationSettings,
		settings.OrganizationID, languages, settings.DefaultApplicationID,
		accountItems, navPreferences, featureFlags, countryCodes, userTypes, tags, mfaItems,
		settings.AccountMenu, settings.MfaRememberInHours, settings.DcrPolicy,
	)
	return err
}

// UpdateTheme upsert organization_theme_settings。
func (r Repository) UpdateTheme(ctx context.Context, theme *domain.OrganizationThemeSettings) error {
	tokens, err := mysqlshared.MarshalJSON(theme.Tokens)
	if err != nil {
		return err
	}
	assets, err := mysqlshared.MarshalJSON(theme.Assets)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryUpsertOrganizationTheme,
		theme.OrganizationID, theme.ThemeName, tokens, theme.CustomCSS, assets,
	)
	return err
}

// UpdateSecurityPolicy upsert organization_security_policies。
func (r Repository) UpdateSecurityPolicy(ctx context.Context, policy *domain.OrganizationSecurityPolicy) error {
	passwordPolicy, err := mysqlshared.MarshalJSON(policy.PasswordPolicy)
	if err != nil {
		return err
	}
	mfaPolicy, err := mysqlshared.MarshalJSON(policy.MfaPolicy)
	if err != nil {
		return err
	}
	ipPolicy, err := mysqlshared.MarshalJSON(policy.IPPolicy)
	if err != nil {
		return err
	}
	sessionPolicy, err := mysqlshared.MarshalJSON(policy.SessionPolicy)
	if err != nil {
		return err
	}
	riskPolicy, err := mysqlshared.MarshalJSON(policy.RiskPolicy)
	if err != nil {
		return err
	}
	lockoutPolicy, err := mysqlshared.MarshalJSON(policy.LockoutPolicy)
	if err != nil {
		return err
	}
	concurrentSessionPolicy, err := mysqlshared.MarshalJSON(policy.ConcurrentSessionPolicy)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryUpsertOrganizationSecurityPolicy,
		policy.OrganizationID, passwordPolicy, policy.EffectivePasswordDeriveAlgorithm(), mfaPolicy, ipPolicy, sessionPolicy, riskPolicy, lockoutPolicy, concurrentSessionPolicy,
	)
	return err
}

// Delete 软删除 organization。
func (r Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteOrganization, id)
	return err
}

// HasDeletedWithSlug 检查同 slug 是否有软删除行。
func (r Repository) HasDeletedWithSlug(ctx context.Context, slug string) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx, queryHasDeletedWithSlug, slug).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
