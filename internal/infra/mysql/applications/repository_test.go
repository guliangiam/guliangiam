package applications

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}

var fixedNow = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

func anyArgs(n int) []driver.Value {
	out := make([]driver.Value, n)
	for i := range out {
		out[i] = sqlmock.AnyArg()
	}
	return out
}

const numApplicationCols = 111

func rowCols() []string {
	out := make([]string, numApplicationCols)
	for i := range out {
		out[i] = "c"
	}
	return out
}

func aggregateQueryPattern(where string) string {
	return "(?s)" + regexp.QuoteMeta("FROM `applications` apps") +
		".*" + regexp.QuoteMeta("LEFT JOIN `app_oauth_config` oauth ON oauth.application_id = apps.id") +
		".*" + regexp.QuoteMeta("LEFT JOIN `app_signin_config` signin ON signin.application_id = apps.id") +
		".*" + regexp.QuoteMeta("LEFT JOIN `app_saml_config` saml ON saml.application_id = apps.id") +
		".*" + regexp.QuoteMeta("LEFT JOIN `app_form_config` form ON form.application_id = apps.id") +
		".*" + regexp.QuoteMeta("LEFT JOIN `app_network_config` net ON net.application_id = apps.id") +
		".*" + regexp.QuoteMeta(where)
}

func upsertPattern(table string) string {
	return "(?s)" + regexp.QuoteMeta("INSERT INTO `"+table+"`") + ".*" + regexp.QuoteMeta("ON DUPLICATE KEY UPDATE")
}

// makeApplicationRow 按 applicationAggregateSelect 的 119 列顺序构造可扫描行。
func makeApplicationRow(id, orgID, slug string) []driver.Value {
	return []driver.Value{
		id,
		orgID,
		slug,
		"Test App",
		"",
		"",
		"",
		"",
		"",
		"active",
		"",
		[]byte("[]"),
		[]byte("[]"),
		"",
		"",
		[]byte("[]"),
		false,
		[]byte("[]"),
		false,
		false,
		"",
		false,
		[]byte("[]"),
		"",
		int64(24),
		int64(168),
		"",
		[]byte("[]"),
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		false,
		false,
		"",
		"",
		false,
		"",
		"",
		int64(0),
		"",
		true,
		false,
		true,
		false,
		int64(168),
		"",
		"",
		"",
		[]byte("[]"),
		[]byte("[]"),
		"",
		int64(0),
		"",
		"",
		false,
		"",
		[]byte("[]"),
		false,
		false,
		false,
		false,
		false,
		"",
		false,
		false,
		"",
		"",
		int64(0),
		int64(0),
		int64(0),
		[]byte("[]"),
		false,
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		false,
		[]byte("[]"),
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		[]byte("[]"),
		[]byte("[]"),
		"",
		fixedNow,
		fixedNow,
		nil,
		nil,
		nil,
		int64(1),
	}
}

func richApplicationRow() []driver.Value {
	vals := makeApplicationRow("app-1", "org-1", "test-app")
	vals[6] = "application description"
	vals[10] = "client-abc"
	vals[11] = []byte(`["https://app.example/callback"]`)
	vals[12] = []byte(`["authorization_code","refresh_token"]`)
	vals[13] = "secret-hash"
	vals[14] = "client_secret_post"
	vals[15] = []byte(`[{"Scope":"profile","DisplayName":"Profile","Description":"Profile scope"}]`)
	vals[17] = []byte(`["tag-a","tag-b"]`)
	vals[22] = []byte(`[{"name":"email","visible":true,"required":true}]`)
	vals[23] = "JWT"
	vals[24] = 24
	vals[25] = 168
	vals[26] = "RS256"
	vals[27] = []byte(`[{"provider":"prov-1","canSignUp":true,"canSignIn":true}]`)
	vals[28] = "https://app.example/saml/acs"
	vals[52] = `[{"claim":"sub"}]`
	vals[53] = []byte(`[{"name":"department","sourceField":"department"}]`)
	vals[60] = []byte(`[{"name":"mail","value":"email"}]`)
	vals[74] = []byte(`["https://a.example","https://b.example"]`)
	vals[78] = ".login{display:grid}"
	vals[94] = []byte(`["login.example","auth.example"]`)
	vals[95] = "upstream.internal"
	vals[96] = "require"
	vals[102] = []byte(`["password","ldap"]`)
	vals[103] = []byte(`[{"name":"username","visible":true}]`)
	vals[110] = int64(7)
	return vals
}

func assertApplicationAggregate(t *testing.T, app *domain.Application) {
	t.Helper()
	assert.Equal(t, "application description", app.Description)
	assert.Equal(t, "client-abc", app.ClientID)
	assert.Equal(t, []string{"https://app.example/callback"}, app.RedirectURIs)
	assert.Equal(t, []string{"authorization_code", "refresh_token"}, app.GrantTypes)
	assert.Equal(t, "secret-hash", app.ClientSecretHash)
	assert.Equal(t, "client_secret_post", app.TokenEndpointAuthMethod)
	assert.Equal(t, []domain.ApplicationCustomScope{{Scope: "profile", DisplayName: "Profile", Description: "Profile scope"}}, app.CustomScopes)
	assert.Equal(t, []string{"tag-a", "tag-b"}, app.Tags)
	assert.Equal(t, []domain.SignupItem{{Name: "email", Visible: true, Required: true}}, app.SignupItems)
	assert.Equal(t, []domain.ProviderItem{{Provider: "prov-1", CanSignUp: true, CanSignIn: true}}, app.ProviderItems)
	assert.Equal(t, "https://app.example/saml/acs", app.SamlReplyUrl)
	assert.Equal(t, `[{"claim":"sub"}]`, app.TokenFields)
	assert.Equal(t, []domain.TokenAttribute{{Name: "department", SourceField: "department"}}, app.TokenAttributes)
	assert.Equal(t, []domain.SamlAttribute{{Name: "mail", Value: "email"}}, app.SamlAttributes)
	assert.Equal(t, "https://a.example,https://b.example", app.CorsAllowedOrigins)
	assert.Equal(t, ".login{display:grid}", app.FormCss)
	assert.Equal(t, []string{"login.example", "auth.example"}, app.OtherDomains)
	assert.Equal(t, "upstream.internal", app.UpstreamHost)
	assert.Equal(t, "require", app.SslMode)
	assert.Equal(t, []string{"password", "ldap"}, app.SigninMethods)
	assert.Equal(t, []domain.SigninItemConfig{{Name: "username", Visible: true}}, app.SigninItems)
	assert.Equal(t, fixedNow, app.CreatedAt)
	assert.Equal(t, fixedNow, app.UpdatedAt)
	assert.Equal(t, int64(7), app.Revision)
}

func testApplication() *domain.Application {
	return &domain.Application{
		ID:                                "app-1",
		OrganizationID:                    "org-1",
		Slug:                              "test-app",
		DisplayName:                       "Test App",
		Category:                          "business",
		AppType:                           "web",
		Description:                       "application description",
		Status:                            "active",
		ClientID:                          "client-abc",
		ClientSecretHash:                  "secret-hash",
		TokenEndpointAuthMethod:           "client_secret_post",
		RedirectURIs:                      []string{"https://app.example/callback"},
		GrantTypes:                        []string{"authorization_code", "refresh_token"},
		CustomScopes:                      []domain.ApplicationCustomScope{{Scope: "profile", DisplayName: "Profile", Description: "Profile scope"}},
		Tags:                              []string{"tag-a", "tag-b"},
		SignupItems:                       []domain.SignupItem{{Name: "email", Visible: true, Required: true}},
		TokenFormat:                       "JWT",
		ExpireInHours:                     24,
		RefreshTokenExpireInHours:         168,
		TokenSigningMethod:                "RS256",
		ProviderItems:                     []domain.ProviderItem{{Provider: "prov-1", CanSignUp: true, CanSignIn: true}},
		TokenFields:                       `[{"claim":"sub"}]`,
		TokenAttributes:                   []domain.TokenAttribute{{Name: "department", SourceField: "department"}},
		SamlAttributes:                    []domain.SamlAttribute{{Name: "mail", Value: "email"}},
		SigninMethods:                     []string{"password", "ldap"},
		SigninItems:                       []domain.SigninItemConfig{{Name: "username", Visible: true}},
		CorsAllowedOrigins:                " https://a.example, ,https://b.example ",
		OtherDomains:                      []string{"login.example", "auth.example"},
		DpopRequired:                      true,
		PkceRequired:                      true,
		CookieExpireInHours:               72,
		EnableBetaOidc:                    true,
		ConsentMode:                       "explicit",
		BackchannelLogoutUri:              "https://app.example/backchannel",
		FrontchannelLogoutUri:             "https://app.example/frontchannel",
		FrontchannelLogoutSessionRequired: true,
		CreatedAt:                         fixedNow,
		UpdatedAt:                         fixedNow,
		Revision:                          1,
	}
}

func expectApplicationConfigWrites(mock sqlmock.Sqlmock, app *domain.Application) {
	oauthArgs := anyArgs(20)
	oauthArgs[0] = app.ID
	oauthArgs[1] = []byte(`["https://app.example/callback"]`)
	oauthArgs[2] = []byte(`["authorization_code","refresh_token"]`)
	oauthArgs[3] = "client_secret_post"
	oauthArgs[4] = "secret-hash"
	oauthArgs[5] = []byte(`[{"Scope":"profile","DisplayName":"Profile","Description":"Profile scope"}]`)
	oauthArgs[9] = app.TokenSigningMethod
	oauthArgs[10] = `[{"claim":"sub"}]`
	oauthArgs[11] = []byte(`[{"name":"department","sourceField":"department"}]`)
	mock.ExpectExec(upsertPattern("app_oauth_config")).WithArgs(oauthArgs...).WillReturnResult(sqlmock.NewResult(1, 1))

	signinArgs := anyArgs(26)
	signinArgs[0] = app.ID
	signinArgs[15] = []byte(`["password","ldap"]`)
	signinArgs[16] = []byte(`[{"name":"username","visible":true}]`)
	signinArgs[25] = []byte(`["https://a.example","https://b.example"]`)
	mock.ExpectExec(upsertPattern("app_signin_config")).WithArgs(signinArgs...).WillReturnResult(sqlmock.NewResult(1, 1))

	samlArgs := anyArgs(17)
	samlArgs[0] = app.ID
	samlArgs[9] = []byte(`[{"name":"mail","value":"email"}]`)
	mock.ExpectExec(upsertPattern("app_saml_config")).WithArgs(samlArgs...).WillReturnResult(sqlmock.NewResult(1, 1))

	formArgs := anyArgs(19)
	formArgs[0] = app.ID
	formArgs[1] = []byte(`[{"name":"email","visible":true,"required":true,"prompted":false}]`)
	mock.ExpectExec(upsertPattern("app_form_config")).WithArgs(formArgs...).WillReturnResult(sqlmock.NewResult(1, 1))

	networkArgs := anyArgs(8)
	networkArgs[0] = app.ID
	networkArgs[1] = []byte(`[{"provider":"prov-1","canSignUp":true,"canSignIn":true,"canUnlink":false}]`)
	networkArgs[3] = []byte(`["login.example","auth.example"]`)
	mock.ExpectExec(upsertPattern("app_network_config")).WithArgs(networkArgs...).WillReturnResult(sqlmock.NewResult(1, 1))
}

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(aggregateQueryPattern("WHERE apps.id = ? AND apps.deleted_at IS NULL")).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows(rowCols()).AddRow(richApplicationRow()...))
	mock.ExpectQuery(regexp.QuoteMeta("FROM `application_provider_bindings` WHERE application_id = ?")).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"application_id", "provider_id"}).
			AddRow("app-1", "prov-1").
			AddRow("app-1", "prov-2"))

	app, err := NewRepository(db).GetByID(context.Background(), "app-1")
	require.NoError(t, err)
	assertApplicationAggregate(t, app)
	assert.Len(t, app.ProviderBindings, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(aggregateQueryPattern("WHERE apps.id = ? AND apps.deleted_at IS NULL")).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows(rowCols()))

	_, err := NewRepository(db).GetByID(context.Background(), "missing")
	var nf *apperrors.Error
	require.True(t, errors.As(err, &nf))
	assert.Equal(t, apperrors.CodeNotFound, nf.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByClientID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(aggregateQueryPattern("WHERE apps.client_id = ? AND apps.deleted_at IS NULL")).
		WithArgs("client-abc").
		WillReturnRows(sqlmock.NewRows(rowCols()).AddRow(richApplicationRow()...))
	mock.ExpectQuery(regexp.QuoteMeta("FROM `application_provider_bindings` WHERE application_id = ?")).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"application_id", "provider_id"}))

	app, err := NewRepository(db).GetByClientID(context.Background(), "client-abc")
	require.NoError(t, err)
	assertApplicationAggregate(t, app)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByOrganizationAndSlug_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(aggregateQueryPattern("WHERE apps.organization_id = ? AND apps.slug = ? AND apps.deleted_at IS NULL")).
		WithArgs("org-1", "test-app").
		WillReturnRows(sqlmock.NewRows(rowCols()).AddRow(richApplicationRow()...))
	mock.ExpectQuery(regexp.QuoteMeta("FROM `application_provider_bindings` WHERE application_id = ?")).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"application_id", "provider_id"}))

	app, err := NewRepository(db).GetByOrganizationAndSlug(context.Background(), "org-1", "test-app")
	require.NoError(t, err)
	assertApplicationAggregate(t, app)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_ReturnsCompleteAggregate(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(aggregateQueryPattern("WHERE (? = '' OR apps.organization_id = ?)")).
		WithArgs("", "", "", "", "", "", "", "", "", "", 20, 0).
		WillReturnRows(sqlmock.NewRows(rowCols()).AddRow(richApplicationRow()...))
	mock.ExpectQuery(regexp.QuoteMeta("FROM `application_provider_bindings` WHERE application_id IN (?)")).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"application_id", "provider_id"}).AddRow("app-1", "prov-1"))
	mock.ExpectQuery("(?s)SELECT COUNT\\(\\*\\).*FROM `applications` apps").
		WithArgs("", "", "", "", "", "", "", "", "", "").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	items, total, err := NewRepository(db).List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assertApplicationAggregate(t, items[0])
	assert.Equal(t, []domain.ApplicationProviderBinding{{ApplicationID: "app-1", ProviderID: "prov-1"}}, items[0].ProviderBindings)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Create_WritesCoreAndAllConfigsInTransaction(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	app := testApplication()

	mock.ExpectBegin()
	mainArgs := anyArgs(23)
	mainArgs[0] = app.ID
	mainArgs[1] = app.OrganizationID
	mainArgs[10] = app.ClientID
	mainArgs[11] = []byte(`["tag-a","tag-b"]`)
	mock.ExpectExec("(?s)" + regexp.QuoteMeta("INSERT INTO `applications`") + ".*" + regexp.QuoteMeta("client_id, tags, logo")).
		WithArgs(mainArgs...).WillReturnResult(sqlmock.NewResult(1, 1))
	expectApplicationConfigWrites(mock, app)
	mock.ExpectCommit()

	err := NewRepository(db).Create(context.Background(), app)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	app := testApplication()
	app.Revision = 2

	mock.ExpectBegin()
	mainArgs := anyArgs(22)
	mainArgs[9] = []byte(`["tag-a","tag-b"]`)
	mainArgs[19] = app.ID
	mainArgs[20] = int64(1)
	mainArgs[21] = app.OrganizationID
	mock.ExpectExec("(?s)" + regexp.QuoteMeta("UPDATE `applications`") + ".*" + regexp.QuoteMeta("revision = revision + 1") + ".*" + regexp.QuoteMeta("WHERE id = ? AND revision = ? AND organization_id = ? AND deleted_at IS NULL")).
		WithArgs(mainArgs...).WillReturnResult(sqlmock.NewResult(1, 1))
	expectApplicationConfigWrites(mock, app)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT revision FROM `applications` WHERE id = ? AND organization_id = ? AND deleted_at IS NULL")).
		WithArgs("app-1", "org-1").
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(3)))
	mock.ExpectCommit()

	err := NewRepository(db).Update(context.Background(), app)
	require.NoError(t, err)
	assert.Equal(t, int64(3), app.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_ConfigFailureRollsBack(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	app := testApplication()
	app.Revision = 2

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `applications`")).WithArgs(anyArgs(22)...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("app_oauth_config")).WithArgs(anyArgs(20)...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("app_signin_config")).WithArgs(anyArgs(26)...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("app_saml_config")).WithArgs(anyArgs(17)...).WillReturnError(errors.New("saml write failed"))
	mock.ExpectRollback()

	err := NewRepository(db).Update(context.Background(), app)
	require.ErrorContains(t, err, "saml write failed")
	assert.Equal(t, int64(2), app.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	app := testApplication()
	app.Revision = 2

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `applications`")).WithArgs(anyArgs(22)...).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := NewRepository(db).Update(context.Background(), app)
	var conflict *apperrors.Error
	require.True(t, errors.As(err, &conflict))
	assert.Equal(t, apperrors.CodeConflict, conflict.Code)
	assert.Equal(t, int64(2), app.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `applications` SET deleted_at = NOW()")).WithArgs("app-1", "org-1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `application_provider_bindings` WHERE application_id = ?")).WithArgs("app-1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `consents` WHERE application_id = ?")).WithArgs("app-1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	require.NoError(t, NewRepository(db).Delete(context.Background(), "app-1", "org-1"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `applications` SET deleted_at = NOW()")).WithArgs("missing", "org-1").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := NewRepository(db).Delete(context.Background(), "missing", "org-1")
	var nf *apperrors.Error
	require.True(t, errors.As(err, &nf))
	assert.Equal(t, apperrors.CodeNotFound, nf.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_ReplaceProviderBindings(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `application_provider_bindings` WHERE application_id = ?")).WithArgs("app-1").WillReturnResult(sqlmock.NewResult(3, 3))
	mock.ExpectExec(regexp.QuoteMeta("INSERT IGNORE INTO `application_provider_bindings`")).WithArgs("app-1", "prov-new").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	bindings := []domain.ApplicationProviderBinding{{ApplicationID: "app-1", ProviderID: "prov-new"}}
	require.NoError(t, NewRepository(db).ReplaceProviderBindings(context.Background(), "app-1", bindings))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_HasDeletedWithOrgAndSlug(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM `applications` WHERE organization_id = ? AND slug = ? AND deleted_at IS NOT NULL)")).
		WithArgs("org-1", "old-slug").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := NewRepository(db).HasDeletedWithOrgAndSlug(context.Background(), "org-1", "old-slug")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_SetRegistrationAccessToken_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec("(?s)"+regexp.QuoteMeta("UPDATE `app_oauth_config` oauth")+".*"+regexp.QuoteMeta("JOIN `applications` apps ON apps.id = oauth.application_id")+".*"+regexp.QuoteMeta("WHERE apps.id = ? AND apps.deleted_at IS NULL")).
		WithArgs("hash-xyz", "app-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, NewRepository(db).SetRegistrationAccessToken(context.Background(), "app-1", "hash-xyz"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetRegistrationAccessTokenHash_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery("(?s)" + regexp.QuoteMeta("SELECT oauth.registration_access_token_hash") + ".*" + regexp.QuoteMeta("JOIN `app_oauth_config` oauth ON oauth.application_id = apps.id") + ".*" + regexp.QuoteMeta("WHERE apps.client_id = ? AND apps.deleted_at IS NULL")).
		WithArgs("client-abc").
		WillReturnRows(sqlmock.NewRows([]string{"registration_access_token_hash"}).AddRow("sha256-hash-value"))

	result, err := NewRepository(db).GetRegistrationAccessTokenHash(context.Background(), "client-abc")
	require.NoError(t, err)
	assert.Equal(t, "sha256-hash-value", result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetRegistrationAccessTokenHash_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT oauth.registration_access_token_hash")).
		WithArgs("client-xyz").
		WillReturnRows(sqlmock.NewRows([]string{"registration_access_token_hash"}).AddRow(nil))

	result, err := NewRepository(db).GetRegistrationAccessTokenHash(context.Background(), "client-xyz")
	require.NoError(t, err)
	assert.Empty(t, result)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindHostConflicts(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT a.id, a.slug")+"(?s).*"+regexp.QuoteMeta("WHERE a.organization_id = ? AND a.deleted_at IS NULL AND a.id <> ?")).
		WithArgs("org-1", "app-self").
		WillReturnRows(sqlmock.NewRows([]string{"id", "slug", "domain", "other_domains"}).
			AddRow("app-2", "other", "App.Example.", []byte(`["https://api.example/path"]`)).
			AddRow("app-3", "unrelated", "unrelated.example", []byte(`[]`)))

	conflicts, err := NewRepository(db).FindHostConflicts(context.Background(), "org-1", "app-self", []string{"app.example", "api.example"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []domain.HostConflict{
		{Host: "app.example", ApplicationID: "app-2", ApplicationSlug: "other"},
		{Host: "api.example", ApplicationID: "app-2", ApplicationSlug: "other"},
	}, conflicts)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindHostConflicts_EmptyHostsSkipQuery(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	conflicts, err := NewRepository(db).FindHostConflicts(context.Background(), "org-1", "app-self", nil)
	require.NoError(t, err)
	assert.Empty(t, conflicts)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_FindHostConflicts_QueryError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery("(?s)SELECT a.id, a.slug.*").
		WithArgs("org-1", "app-self").
		WillReturnError(errors.New("db down"))

	_, err := NewRepository(db).FindHostConflicts(context.Background(), "org-1", "app-self", []string{"app.example"})
	require.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

var _ domain.Repository = Repository{}
