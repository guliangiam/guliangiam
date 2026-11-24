package organizations

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	_ = os.Setenv("ENCRYPTION_KEY_CURRENT", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
}

var (
	fixedNow  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID = "00000000-0000-0000-0000-000000000001"
	testSlug  = "test-org"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}

func anyArgs(n int) []driver.Value {
	out := make([]driver.Value, n)
	for i := range out {
		out[i] = sqlmock.AnyArg()
	}
	return out
}

func orgAggCols() []string {
	return []string{
		"o.id", "o.slug", "o.display_name", "o.website_url", "o.logo_url", "o.logo_dark_url", "o.favicon_url",
		"o.status", "o.init_score", "o.default_avatar", "ac.password_obfuscator_type", "o.disable_signin",
		"ac.password_options", "o.enable_soft_deletion", "o.disable_signup", "o.ip_whitelist",
		"o.initiate_login_url", "bl.balance", "bl.currency",
		"ac.password_type", "ac.password_salt", "ac.password_obfuscator_key", "bl.enable_deposit", "bl.balance_credit",
		"bl.mu", "bl.default_quota", "bl.default_currency", "br.enable_dark_logo", "br.has_privilege_consent",
		"br.is_profile_public", "ac.use_email_as_username", "br.use_permanent_avatar", "br.enable_tour",
		"ac.master_password", "ac.master_verification_code", "ac.default_password", "ac.ldap_attributes",
		"kb.kerberos_realm", "kb.kerberos_kdc_host", "kb.kerberos_keytab", "kb.kerberos_service_name",
		"o.created_at", "o.updated_at", "o.revision",
		"s.organization_id", "s.languages", "s.default_application_id", "s.account_items",
		"s.nav_preferences", "s.feature_flags", "s.country_codes", "s.user_types", "s.tags",
		"s.mfa_items", "s.account_menu", "s.mfa_remember_in_hours", "s.dcr_policy",
		"t.organization_id", "t.theme_name", "t.tokens", "t.custom_css", "t.assets",
		"p.organization_id", "p.password_policy", "p.password_derive_algorithm", "p.mfa_policy", "p.ip_policy",
		"p.session_policy", "p.risk_policy", "p.lockout_policy", "p.concurrent_session_policy",
		"o.parent_id",
		"o.region_code",
	}
}

func organizationAggregatePattern(where string) string {
	return "(?s)" + regexp.QuoteMeta("FROM `organizations` o") +
		".*" + regexp.QuoteMeta("LEFT JOIN `organization_settings` s ON s.organization_id = o.id") +
		".*" + regexp.QuoteMeta("LEFT JOIN `organization_theme_settings` t ON t.organization_id = o.id") +
		".*" + regexp.QuoteMeta("LEFT JOIN `organization_security_policies` p ON p.organization_id = o.id") +
		".*" + regexp.QuoteMeta("LEFT JOIN `organization_auth_config` ac ON ac.organization_id = o.id") +
		".*" + regexp.QuoteMeta("LEFT JOIN `organization_kerberos_config` kb ON kb.organization_id = o.id") +
		".*" + regexp.QuoteMeta("LEFT JOIN `organization_billing` bl ON bl.organization_id = o.id") +
		".*" + regexp.QuoteMeta("LEFT JOIN `organization_branding` br ON br.organization_id = o.id") +
		".*" + regexp.QuoteMeta(where)
}

func upsertPattern(table string) string {
	return "(?s)" + regexp.QuoteMeta("INSERT INTO `"+table+"`") + ".*" + regexp.QuoteMeta("ON DUPLICATE KEY UPDATE")
}

func newOrgBaseRow() []driver.Value {
	row := []driver.Value{
		testOrgID, testSlug, "Test Org", "https://example.com", "https://example.com/logo.png",
		"https://example.com/logo-dark.png", "https://example.com/favicon.ico", "active", 10,
		"https://example.com/avatar.png", "plain", false, []byte(`["minLength","requireUpper"]`),
		true, false, "10.0.0.0/8", "https://example.com/initiate", 12.5, "USD",
		"argon2id", "password-salt", "obfuscator-key", true, 3.5, 2, 100, "CNY",
		true, true, true, true, true, true, "", "", "", []byte(`["cn","mail"]`),
		"EXAMPLE.COM", "kdc.example.com", "keytab-data", "HTTP/host.example.com",
		fixedNow, fixedNow, int64(4),
	}
	return row
}

func newOrgAggRow(withRelations bool) []driver.Value {
	row := newOrgBaseRow()
	if !withRelations {
		return append(row,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			"",
		)
	}
	return append(row,
		testOrgID,
		[]byte(`["en","zh"]`),
		"app-1",
		[]byte(`[{"name":"email","visible":true,"viewRule":"self","modifyRule":"self","regex":"","tab":"account"}]`),
		[]byte(`{"sidebar":true}`),
		[]byte(`{"beta":true}`),
		[]byte(`["CN","US"]`),
		[]byte(`["admin","member"]`),
		[]byte(`["enterprise"]`),
		[]byte(`[{"name":"totp","rule":"required"}]`),
		"vertical",
		24,
		"trusted",
		testOrgID,
		"dark",
		[]byte(`{"primaryColor":"#ffffff"}`),
		".org{display:grid}",
		[]byte(`{"logo":"https://example.com/logo.svg"}`),
		testOrgID,
		[]byte(`{"minLength":12}`),
		"hmac_sha256",
		[]byte(`{"required":true}`),
		[]byte(`{"mode":"enforce"}`),
		[]byte(`{"maxAge":3600}`),
		[]byte(`{"level":"medium"}`),
		[]byte(`{"maxAttempts":5}`),
		[]byte(`{"maxSessions":3}`),
		testOrgID,
		"cn-north",
	)
}

func testOrganization() *domain.Organization {
	return &domain.Organization{
		ID:                     testOrgID,
		ParentID:               testOrgID,
		RegionCode:             "cn-north",
		Slug:                   testSlug,
		DisplayName:            "Test Org",
		WebsiteURL:             "https://example.com",
		LogoURL:                "https://example.com/logo.png",
		LogoDarkURL:            "https://example.com/logo-dark.png",
		FaviconURL:             "https://example.com/favicon.ico",
		Status:                 "active",
		InitScore:              10,
		DefaultAvatar:          "https://example.com/avatar.png",
		PasswordObfuscatorType: "plain",
		PasswordOptions:        []string{"minLength", "requireUpper"},
		EnableSoftDeletion:     true,
		IpWhitelist:            "10.0.0.0/8",
		InitiateLoginURL:       "https://example.com/initiate",
		Balance:                12.5,
		Currency:               "USD",
		PasswordType:           "argon2id",
		PasswordSalt:           "password-salt",
		PasswordObfuscatorKey:  "obfuscator-key",
		EnableDeposit:          true,
		BalanceCredit:          3.5,
		Mu:                     2,
		DefaultQuota:           100,
		DefaultCurrency:        "CNY",
		EnableDarkLogo:         true,
		HasPrivilegeConsent:    true,
		IsProfilePublic:        true,
		UseEmailAsUsername:     true,
		UsePermanentAvatar:     true,
		EnableTour:             true,
		LdapAttributes:         []string{"cn", "mail"},
		KerberosRealm:          "EXAMPLE.COM",
		KerberosKdcHost:        "kdc.example.com",
		KerberosKeytab:         "keytab-data",
		KerberosServiceName:    "HTTP/host.example.com",
		CreatedAt:              fixedNow,
		UpdatedAt:              fixedNow,
		Revision:               1,
		Settings: domain.OrganizationSettings{
			OrganizationID:       testOrgID,
			Languages:            []string{"en", "zh"},
			DefaultApplicationID: "app-1",
			AccountItems:         []domain.AccountItem{{Name: "email", Visible: true, ViewRule: "self", ModifyRule: "self", Tab: "account"}},
			NavPreferences:       map[string]any{"sidebar": true},
			FeatureFlags:         map[string]any{"beta": true},
			CountryCodes:         []string{"CN", "US"},
			UserTypes:            []string{"admin", "member"},
			Tags:                 []string{"enterprise"},
			MfaItems:             []domain.MfaItem{{Name: "totp", Rule: "required"}},
			AccountMenu:          "vertical",
			MfaRememberInHours:   24,
			DcrPolicy:            "trusted",
		},
		Theme: domain.OrganizationThemeSettings{
			OrganizationID: testOrgID,
			ThemeName:      "dark",
			Tokens:         map[string]any{"primaryColor": "#ffffff"},
			CustomCSS:      ".org{display:grid}",
			Assets:         map[string]any{"logo": "https://example.com/logo.svg"},
		},
		Security: domain.OrganizationSecurityPolicy{
			OrganizationID:          testOrgID,
			PasswordPolicy:          map[string]any{"minLength": float64(12)},
			PasswordDeriveAlgorithm: domain.PasswordDeriveAlgorithmHMACSHA256,
			MfaPolicy:               map[string]any{"required": true},
			IPPolicy:                map[string]any{"mode": "enforce"},
			SessionPolicy:           map[string]any{"maxAge": float64(3600)},
			RiskPolicy:              map[string]any{"level": "medium"},
			LockoutPolicy:           map[string]any{"maxAttempts": float64(5)},
			ConcurrentSessionPolicy: map[string]any{"maxSessions": float64(3)},
		},
	}
}

func assertOrganizationAggregate(t *testing.T, org *domain.Organization) {
	t.Helper()
	expected := testOrganization()
	assert.Equal(t, expected.ID, org.ID)
	assert.Equal(t, expected.Slug, org.Slug)
	assert.Equal(t, expected.DisplayName, org.DisplayName)
	assert.Equal(t, expected.PasswordObfuscatorType, org.PasswordObfuscatorType)
	assert.Equal(t, expected.PasswordOptions, org.PasswordOptions)
	assert.Equal(t, expected.EnableSoftDeletion, org.EnableSoftDeletion)
	assert.Equal(t, expected.IpWhitelist, org.IpWhitelist)
	assert.Equal(t, expected.InitiateLoginURL, org.InitiateLoginURL)
	assert.Equal(t, expected.Balance, org.Balance)
	assert.Equal(t, expected.Currency, org.Currency)
	assert.Equal(t, expected.PasswordType, org.PasswordType)
	assert.Equal(t, expected.PasswordSalt, org.PasswordSalt)
	assert.Equal(t, expected.PasswordObfuscatorKey, org.PasswordObfuscatorKey)
	assert.Equal(t, expected.EnableDeposit, org.EnableDeposit)
	assert.Equal(t, expected.BalanceCredit, org.BalanceCredit)
	assert.Equal(t, expected.Mu, org.Mu)
	assert.Equal(t, expected.DefaultQuota, org.DefaultQuota)
	assert.Equal(t, expected.DefaultCurrency, org.DefaultCurrency)
	assert.Equal(t, expected.EnableDarkLogo, org.EnableDarkLogo)
	assert.Equal(t, expected.HasPrivilegeConsent, org.HasPrivilegeConsent)
	assert.Equal(t, expected.IsProfilePublic, org.IsProfilePublic)
	assert.Equal(t, expected.UseEmailAsUsername, org.UseEmailAsUsername)
	assert.Equal(t, expected.UsePermanentAvatar, org.UsePermanentAvatar)
	assert.Equal(t, expected.EnableTour, org.EnableTour)
	assert.Equal(t, expected.LdapAttributes, org.LdapAttributes)
	assert.Equal(t, expected.KerberosRealm, org.KerberosRealm)
	assert.Equal(t, expected.KerberosKdcHost, org.KerberosKdcHost)
	assert.Equal(t, expected.KerberosKeytab, org.KerberosKeytab)
	assert.Equal(t, expected.KerberosServiceName, org.KerberosServiceName)
	assert.Equal(t, expected.Settings, org.Settings)
	assert.Equal(t, expected.Theme, org.Theme)
	assert.Equal(t, expected.Security, org.Security)
	assert.Equal(t, fixedNow, org.CreatedAt)
	assert.Equal(t, fixedNow, org.UpdatedAt)
	assert.Equal(t, int64(4), org.Revision)
}

func expectOrganizationConfigWrites(mock sqlmock.Sqlmock, org *domain.Organization) {
	authArgs := anyArgs(11)
	authArgs[0] = org.ID
	authArgs[1] = org.PasswordObfuscatorType
	authArgs[2] = []byte(`["minLength","requireUpper"]`)
	authArgs[9] = org.UseEmailAsUsername
	authArgs[10] = []byte(`["cn","mail"]`)
	mock.ExpectExec(upsertPattern("organization_auth_config")).WithArgs(authArgs...).WillReturnResult(sqlmock.NewResult(1, 1))

	kerberosArgs := anyArgs(5)
	kerberosArgs[0] = org.ID
	kerberosArgs[1] = org.KerberosRealm
	kerberosArgs[2] = org.KerberosKdcHost
	kerberosArgs[3] = org.KerberosKeytab
	kerberosArgs[4] = org.KerberosServiceName
	mock.ExpectExec(upsertPattern("organization_kerberos_config")).WithArgs(kerberosArgs...).WillReturnResult(sqlmock.NewResult(1, 1))

	billingArgs := anyArgs(8)
	billingArgs[0] = org.ID
	billingArgs[1] = org.Balance
	billingArgs[2] = org.Currency
	billingArgs[3] = org.EnableDeposit
	billingArgs[4] = org.BalanceCredit
	billingArgs[5] = org.Mu
	billingArgs[6] = org.DefaultQuota
	billingArgs[7] = org.DefaultCurrency
	mock.ExpectExec(upsertPattern("organization_billing")).WithArgs(billingArgs...).WillReturnResult(sqlmock.NewResult(1, 1))

	brandingArgs := anyArgs(6)
	brandingArgs[0] = org.ID
	brandingArgs[1] = org.EnableDarkLogo
	brandingArgs[2] = org.HasPrivilegeConsent
	brandingArgs[3] = org.IsProfilePublic
	brandingArgs[4] = org.UsePermanentAvatar
	brandingArgs[5] = org.EnableTour
	mock.ExpectExec(upsertPattern("organization_branding")).WithArgs(brandingArgs...).WillReturnResult(sqlmock.NewResult(1, 1))
}

func TestRowColumnCounts(t *testing.T) {
	assert.Len(t, newOrgBaseRow(), 44)
	assert.Len(t, newOrgAggRow(true), 73)
	assert.Len(t, newOrgAggRow(false), 73)
	assert.Len(t, orgAggCols(), 73)
}

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(organizationAggregatePattern("WHERE o.id = ? AND o.deleted_at IS NULL")).
		WithArgs(testOrgID).
		WillReturnRows(sqlmock.NewRows(orgAggCols()).AddRow(newOrgAggRow(true)...))

	org, err := NewRepository(db).GetByID(context.Background(), testOrgID)
	require.NoError(t, err)
	assertOrganizationAggregate(t, org)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(organizationAggregatePattern("WHERE o.id = ? AND o.deleted_at IS NULL")).
		WithArgs("unknown-id").
		WillReturnRows(sqlmock.NewRows(orgAggCols()))

	org, err := NewRepository(db).GetByID(context.Background(), "unknown-id")
	assert.Error(t, err)
	assert.Nil(t, org)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetBySlug_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(organizationAggregatePattern("WHERE o.slug = ? AND o.deleted_at IS NULL")).
		WithArgs(testSlug).
		WillReturnRows(sqlmock.NewRows(orgAggCols()).AddRow(newOrgAggRow(true)...))

	org, err := NewRepository(db).GetBySlug(context.Background(), testSlug)
	require.NoError(t, err)
	assertOrganizationAggregate(t, org)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_ReturnsCompleteAggregate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(organizationAggregatePattern("WHERE (? = '' OR o.slug LIKE CONCAT('%', ?, '%') OR o.display_name LIKE CONCAT('%', ?, '%'))")).
		WithArgs("", "", "", "", "", 20, 0).
		WillReturnRows(sqlmock.NewRows(orgAggCols()).AddRow(newOrgAggRow(true)...))
	mock.ExpectQuery("(?s)SELECT COUNT\\(\\*\\).*FROM `organizations` o").
		WithArgs("", "", "", "", "").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	items, total, err := NewRepository(db).List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assertOrganizationAggregate(t, items[0])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_WithKeyword(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(organizationAggregatePattern("WHERE (? = '' OR o.slug LIKE CONCAT('%', ?, '%') OR o.display_name LIKE CONCAT('%', ?, '%'))")).
		WithArgs("test", "test", "test", "", "", 20, 0).
		WillReturnRows(sqlmock.NewRows(orgAggCols()))
	mock.ExpectQuery("(?s)SELECT COUNT\\(\\*\\).*FROM `organizations` o").
		WithArgs("test", "test", "test", "", "").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	items, total, err := NewRepository(db).List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20, Keyword: "test"})
	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Zero(t, total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Create_WritesCoreAndAllConfigsInTransaction(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	org := testOrganization()

	mock.ExpectBegin()
	mainArgs := anyArgs(20)
	mainArgs[0] = org.ID
	mainArgs[1] = org.Slug
	mainArgs[2] = org.DisplayName
	mainArgs[15] = org.ParentID
	mainArgs[16] = org.CreatedAt
	mainArgs[17] = org.UpdatedAt
	mainArgs[18] = org.Revision
	mainArgs[19] = org.RegionCode
	mock.ExpectExec("(?s)" + regexp.QuoteMeta("INSERT INTO `organizations`") + ".*" + regexp.QuoteMeta("disable_signup, ip_whitelist, initiate_login_url, created_at, updated_at, revision")).
		WithArgs(mainArgs...).WillReturnResult(sqlmock.NewResult(1, 1))
	expectOrganizationConfigWrites(mock, org)
	mock.ExpectCommit()

	require.NoError(t, NewRepository(db).Create(context.Background(), org))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	org := testOrganization()
	org.Revision = 2

	mock.ExpectBegin()
	mainArgs := anyArgs(18)
	mainArgs[0] = org.DisplayName
	mainArgs[13] = org.ParentID
	mainArgs[14] = org.UpdatedAt
	mainArgs[15] = org.RegionCode
	mainArgs[16] = org.ID
	mainArgs[17] = int64(1)
	mock.ExpectExec("(?s)" + regexp.QuoteMeta("UPDATE `organizations`") + ".*" + regexp.QuoteMeta("revision = revision + 1") + ".*" + regexp.QuoteMeta("WHERE id = ? AND revision = ? AND deleted_at IS NULL")).
		WithArgs(mainArgs...).WillReturnResult(sqlmock.NewResult(1, 1))
	expectOrganizationConfigWrites(mock, org)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT revision FROM `organizations` WHERE id = ? AND deleted_at IS NULL")).
		WithArgs(testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(3)))
	mock.ExpectCommit()

	require.NoError(t, NewRepository(db).Update(context.Background(), org))
	assert.Equal(t, int64(3), org.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_ConfigFailureRollsBack(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	org := testOrganization()
	org.Revision = 2

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `organizations`")).WithArgs(anyArgs(18)...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("organization_auth_config")).WithArgs(anyArgs(11)...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("organization_kerberos_config")).WithArgs(anyArgs(5)...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("organization_billing")).WithArgs(anyArgs(8)...).WillReturnError(errors.New("billing write failed"))
	mock.ExpectRollback()

	err := NewRepository(db).Update(context.Background(), org)
	require.ErrorContains(t, err, "billing write failed")
	assert.Equal(t, int64(2), org.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	org := testOrganization()
	org.Revision = 99

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `organizations`")).WithArgs(anyArgs(18)...).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := NewRepository(db).Update(context.Background(), org)
	var conflict *apperrors.Error
	require.True(t, errors.As(err, &conflict))
	assert.Equal(t, apperrors.CodeConflict, conflict.Code)
	assert.Equal(t, int64(99), org.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_SettingsThemeSecurityPolicy_RoundTrip(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	org := testOrganization()

	mock.ExpectExec(upsertPattern("organization_settings")).WithArgs(
		testOrgID,
		[]byte(`["en","zh"]`),
		"app-1",
		[]byte(`[{"name":"email","visible":true,"viewRule":"self","modifyRule":"self","regex":"","tab":"account"}]`),
		[]byte(`{"sidebar":true}`),
		[]byte(`{"beta":true}`),
		[]byte(`["CN","US"]`),
		[]byte(`["admin","member"]`),
		[]byte(`["enterprise"]`),
		[]byte(`[{"name":"totp","rule":"required"}]`),
		"vertical",
		24,
		"trusted",
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("organization_theme_settings")).WithArgs(
		testOrgID,
		"dark",
		[]byte(`{"primaryColor":"#ffffff"}`),
		".org{display:grid}",
		[]byte(`{"logo":"https://example.com/logo.svg"}`),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("organization_security_policies")).WithArgs(
		testOrgID,
		[]byte(`{"minLength":12}`),
		"hmac_sha256",
		[]byte(`{"required":true}`),
		[]byte(`{"mode":"enforce"}`),
		[]byte(`{"maxAge":3600}`),
		[]byte(`{"level":"medium"}`),
		[]byte(`{"maxAttempts":5}`),
		[]byte(`{"maxSessions":3}`),
	).WillReturnResult(sqlmock.NewResult(1, 1))

	repo := NewRepository(db)
	require.NoError(t, repo.UpdateSettings(context.Background(), &org.Settings))
	require.NoError(t, repo.UpdateTheme(context.Background(), &org.Theme))
	require.NoError(t, repo.UpdateSecurityPolicy(context.Background(), &org.Security))

	mock.ExpectQuery(organizationAggregatePattern("WHERE o.id = ? AND o.deleted_at IS NULL")).
		WithArgs(testOrgID).
		WillReturnRows(sqlmock.NewRows(orgAggCols()).AddRow(newOrgAggRow(true)...))
	loaded, err := repo.GetByID(context.Background(), testOrgID)
	require.NoError(t, err)
	assert.Equal(t, org.Settings, loaded.Settings)
	assert.Equal(t, org.Theme, loaded.Theme)
	assert.Equal(t, org.Security, loaded.Security)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteOrganization)).WithArgs(testOrgID).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, NewRepository(db).Delete(context.Background(), testOrgID))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_HasDeletedWithSlug_Yes(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryHasDeletedWithSlug)).
		WithArgs("old").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := NewRepository(db).HasDeletedWithSlug(context.Background(), "old")
	require.NoError(t, err)
	assert.True(t, exists)
	require.NoError(t, mock.ExpectationsWereMet())
}

var _ domain.Repository = Repository{}

func TestOrganizationParentIDEmptyStringIsNormalizedMySQL(t *testing.T) {
	t.Parallel()
	if !strings.Contains(queryUpdateOrganization, "parent_id = NULLIF(?, '')") {
		t.Fatal("update must normalize an empty parent_id to NULL")
	}
}
