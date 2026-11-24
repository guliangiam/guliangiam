package applications

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// Empty entity value must not overwrite the column default: the upsert
// expression coalesces it to 'RS256' instead of writing ” (GM N8a).
func TestUpsertOAuthConfig_CoalescesEmptyTokenSigningMethod(t *testing.T) {
	require.Contains(t, queryUpsertOAuthConfig, "COALESCE(NULLIF(?, ''), 'RS256')")
}

func TestRepository_Create_EmptyTokenSigningMethodUsesSQLDefault(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	app := testApplication()
	app.TokenSigningMethod = ""

	mock.ExpectBegin()
	mainArgs := anyArgs(23)
	mainArgs[0] = app.ID
	mainArgs[1] = app.OrganizationID
	mainArgs[10] = app.ClientID
	mainArgs[11] = []byte(`["tag-a","tag-b"]`)
	mock.ExpectExec("(?s)" + regexp.QuoteMeta("INSERT INTO `applications`") + ".*" + regexp.QuoteMeta("client_id, tags, logo")).
		WithArgs(mainArgs...).WillReturnResult(sqlmock.NewResult(1, 1))

	oauthArgs := anyArgs(20)
	oauthArgs[0] = app.ID
	oauthArgs[1] = []byte(`["https://app.example/callback"]`)
	oauthArgs[2] = []byte(`["authorization_code","refresh_token"]`)
	oauthArgs[3] = "client_secret_post"
	oauthArgs[4] = "secret-hash"
	oauthArgs[5] = []byte(`[{"Scope":"profile","DisplayName":"Profile","Description":"Profile scope"}]`)
	oauthArgs[9] = ""
	oauthArgs[10] = `[{"claim":"sub"}]`
	oauthArgs[11] = []byte(`[{"name":"department","sourceField":"department"}]`)
	mock.ExpectExec("(?s)" + regexp.QuoteMeta("INSERT INTO `app_oauth_config`") +
		".*" + regexp.QuoteMeta("COALESCE(NULLIF(?, ''), 'RS256')") +
		".*" + regexp.QuoteMeta("ON DUPLICATE KEY UPDATE")).
		WithArgs(oauthArgs...).WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(upsertPattern("app_signin_config")).WithArgs(anyArgs(26)...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("app_saml_config")).WithArgs(anyArgs(17)...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("app_form_config")).WithArgs(anyArgs(19)...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(upsertPattern("app_network_config")).WithArgs(anyArgs(8)...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	require.NoError(t, NewRepository(db).Create(context.Background(), app))
	require.NoError(t, mock.ExpectationsWereMet())
}
